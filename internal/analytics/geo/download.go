package geo

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/oschwald/maxminddb-golang/v2"
)

const (
	maxArchiveBytes  = int64(1) << 30
	maxDatabaseBytes = int64(1) << 30
	checksumBytes    = 1024
	sha256HexLength  = 64
)

func (u *Updater) endpoint(suffix string) string {
	return u.base + "/geoip/databases/" + editionID + "/download?suffix=" + suffix
}

func (u *Updater) get(ctx context.Context, address, account, key string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, fmt.Errorf("build the MaxMind request: %w", err)
	}
	req.SetBasicAuth(account, key)
	resp, err := u.client.Do(req)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return nil, fmt.Errorf("could not reach MaxMind: %w", err)
	}
	return resp, nil
}

type plainReason string

func (r plainReason) Error() string {
	return string(r)
}

func statusProblem(code int) error {
	switch code {
	case http.StatusUnauthorized:
		return plainReason("MaxMind rejected the account ID or licence key. A key created in the last few minutes may not be active yet; wait a few minutes and check again.")
	case http.StatusForbidden:
		return plainReason("MaxMind does not allow this account to download GeoLite2-City (HTTP 403). Check that the account has accepted the GeoLite2 terms.")
	case http.StatusTooManyRequests:
		return plainReason("MaxMind's download limit for this account was reached (HTTP 429). Wait a while and check again.")
	default:
		return fmt.Errorf("MaxMind returned HTTP %d", code)
	}
}

func (u *Updater) fetchArchive(ctx context.Context, account, key string) (*os.File, []byte, error) {
	resp, err := u.get(ctx, u.endpoint("tar.gz"), account, key)
	if err != nil {
		return nil, nil, err
	}
	defer closeBody(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, nil, statusProblem(resp.StatusCode)
	}
	file, err := os.CreateTemp(u.dir, "download-*.tar.gz")
	if err != nil {
		return nil, nil, fmt.Errorf("create a temporary download file: %w", err)
	}
	hasher := sha256.New()
	limited := &io.LimitedReader{R: resp.Body, N: maxArchiveBytes + 1}
	written, copyErr := io.Copy(io.MultiWriter(file, hasher), limited)
	if err := firstProblem(copyErr, nil, written > maxArchiveBytes); err != nil {
		discard(file)
		return nil, nil, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		discard(file)
		return nil, nil, fmt.Errorf("rewind the temporary download file: %w", err)
	}
	return file, hasher.Sum(nil), nil
}

func discard(file *os.File) {
	closeBody(file)
	removeQuietly(file.Name())
}

func firstProblem(copyErr, closeErr error, tooLarge bool) error {
	switch {
	case copyErr != nil:
		return errors.New("the download was interrupted")
	case closeErr != nil:
		return fmt.Errorf("write the temporary download file: %w", closeErr)
	case tooLarge:
		return errors.New("the download is larger than the allowed size")
	default:
		return nil
	}
}

func (u *Updater) fetchChecksum(ctx context.Context, account, key string) (string, bool, error) {
	resp, err := u.get(ctx, u.endpoint("tar.gz.sha256"), account, key)
	if err != nil {
		return "", false, err
	}
	defer closeBody(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return "", false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", false, statusProblem(resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, checksumBytes))
	if err != nil {
		return "", false, errors.New("the checksum download was interrupted")
	}
	fields := strings.Fields(string(body))
	if len(fields) == 0 || len(fields[0]) != sha256HexLength {
		return "", false, errors.New("MaxMind's checksum file is not in the expected format")
	}
	if _, err := hex.DecodeString(fields[0]); err != nil {
		return "", false, errors.New("MaxMind's checksum file is not in the expected format")
	}
	return strings.ToLower(fields[0]), true, nil
}

func closeBody(body io.Closer) {
	if err := body.Close(); err != nil {
		slog.Debug("close a geo download stream", "error", err)
	}
}

func (u *Updater) install(archive *os.File) error {
	staged, err := u.extract(archive)
	if err != nil {
		return err
	}
	if err := validateDatabase(staged); err != nil {
		removeQuietly(staged)
		return err
	}
	if err := os.Rename(staged, filepath.Join(u.dir, FileName)); err != nil {
		removeQuietly(staged)
		return fmt.Errorf("move the database into place: %w", err)
	}
	return nil
}

func validateDatabase(file string) error {
	reader, err := maxminddb.Open(file)
	if err != nil {
		return errors.New("the downloaded database is not a valid MaxMind database")
	}
	if err := reader.Close(); err != nil {
		return fmt.Errorf("close the staged database: %w", err)
	}
	return nil
}

func (u *Updater) extract(archive *os.File) (string, error) {
	unzipped, err := gzip.NewReader(archive)
	if err != nil {
		return "", errors.New("the download is not a gzip archive")
	}
	defer closeBody(unzipped)
	reader := tar.NewReader(unzipped)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return "", fmt.Errorf("the archive does not contain %s", FileName)
		}
		if err != nil {
			return "", errors.New("the archive is damaged")
		}
		if header.Typeflag == tar.TypeReg && path.Base(header.Name) == FileName {
			return u.stage(reader)
		}
	}
}

func (u *Updater) stage(source io.Reader) (string, error) {
	file, err := os.CreateTemp(u.dir, "staged-*.mmdb")
	if err != nil {
		return "", fmt.Errorf("create a staging file: %w", err)
	}
	limited := &io.LimitedReader{R: source, N: maxDatabaseBytes + 1}
	written, copyErr := io.Copy(file, limited)
	closeErr := file.Close()
	if err := firstProblem(copyErr, closeErr, written > maxDatabaseBytes); err != nil {
		removeQuietly(file.Name())
		return "", err
	}
	return file.Name(), nil
}
