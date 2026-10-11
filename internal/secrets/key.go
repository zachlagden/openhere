package secrets

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/zachlagden/openhere/internal/fsperm"
)

const (
	KeyFileName = "secret.key"
	fileMode    = 0o600
)

func ParseKey(raw string) ([]byte, error) {
	trimmed := strings.TrimSpace(raw)
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		decoded, err := encoding.DecodeString(trimmed)
		if err == nil && len(decoded) == KeyBytes {
			return decoded, nil
		}
	}
	return nil, fmt.Errorf("the secret key must be %d random bytes encoded as base64", KeyBytes)
}

func GenerateKey() (string, error) {
	key := make([]byte, KeyBytes)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("generate secret key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

func Resolve(envKey []byte, dataDir string) (*Box, error) {
	if len(envKey) > 0 {
		return New(envKey)
	}
	if err := fsperm.EnsureDir(dataDir, nil); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	root, err := os.OpenRoot(dataDir)
	if err != nil {
		return nil, fmt.Errorf("open data directory: %w", err)
	}
	key, err := loadOrCreateFile(root)
	if closeErr := root.Close(); closeErr != nil && err == nil {
		err = fmt.Errorf("close data directory: %w", closeErr)
	}
	if err != nil {
		return nil, err
	}
	return New(key)
}

func loadOrCreateFile(root *os.Root) ([]byte, error) {
	existing, err := readKeyFile(root)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	encoded, err := GenerateKey()
	if err != nil {
		return nil, err
	}
	file, err := root.OpenFile(KeyFileName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fileMode)
	if errors.Is(err, fs.ErrExist) {
		return readKeyFile(root)
	}
	if err != nil {
		return nil, fmt.Errorf("create secret key file: %w", err)
	}
	if _, err := file.WriteString(encoded + "\n"); err != nil {
		return nil, errors.Join(fmt.Errorf("write secret key file: %w", err), file.Close())
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close secret key file: %w", err)
	}
	return ParseKey(encoded)
}

func readKeyFile(root *os.Root) ([]byte, error) {
	file, err := root.Open(KeyFileName)
	if err != nil {
		return nil, fmt.Errorf("open secret key file: %w", err)
	}
	content, err := io.ReadAll(io.LimitReader(file, 4096))
	if closeErr := file.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, fmt.Errorf("read secret key file: %w", err)
	}
	key, err := ParseKey(string(content))
	if err != nil {
		return nil, fmt.Errorf("secret key file %s: %w", KeyFileName, err)
	}
	return key, nil
}
