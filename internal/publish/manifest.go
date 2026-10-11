package publish

import (
	"fmt"
	"mime"
	"path"
	"strings"
	"unicode"

	"github.com/zachlagden/openhere/internal/settings"
	"github.com/zachlagden/openhere/internal/sitepath"
	"github.com/zachlagden/openhere/internal/validate"
)

const (
	maxContentTypeLength = 255
	fallbackContentType  = "application/octet-stream"
	sha256HexLength      = 64
)

func prepareManifest(specs []FileSpec, cfg settings.Settings) ([]FileSpec, error) {
	if len(specs) == 0 {
		return nil, validate.Invalid("Files must list at least one file.")
	}
	if len(specs) > cfg.MaxFilesPerVer {
		return nil, ErrTooManyFiles
	}
	paths := make([]string, len(specs))
	prepared := make([]FileSpec, len(specs))
	for i, spec := range specs {
		clean, err := prepareSpec(spec)
		if err != nil {
			return nil, err
		}
		paths[i] = clean.Path
		prepared[i] = clean
	}
	if err := sitepath.CheckSet(paths); err != nil {
		return nil, err
	}
	if err := checkLimits(prepared, cfg); err != nil {
		return nil, err
	}
	return prepared, nil
}

func checkLimits(specs []FileSpec, cfg settings.Settings) error {
	if len(specs) > cfg.MaxFilesPerVer {
		return ErrTooManyFiles
	}
	var total int64
	for _, spec := range specs {
		if spec.Size > cfg.MaxFileBytes {
			return fmt.Errorf("%w: %q", ErrFileTooLarge, spec.Path)
		}
		total += spec.Size
		if total > cfg.MaxVersionBytes {
			return ErrVersionTooLarge
		}
	}
	return nil
}

func prepareSpec(spec FileSpec) (FileSpec, error) {
	if err := sitepath.Validate(spec.Path); err != nil {
		return FileSpec{}, err
	}
	if spec.Size < 0 {
		return FileSpec{}, validate.Invalid(fmt.Sprintf("File %q has a negative size.", spec.Path))
	}
	hash, err := normaliseHash(spec.Path, spec.SHA256)
	if err != nil {
		return FileSpec{}, err
	}
	contentType, err := normaliseContentType(spec.Path, spec.ContentType)
	if err != nil {
		return FileSpec{}, err
	}
	return FileSpec{Path: spec.Path, Size: spec.Size, SHA256: hash, ContentType: contentType}, nil
}

func normaliseHash(filePath, raw string) (string, error) {
	hash := strings.ToLower(raw)
	if len(hash) != sha256HexLength {
		return "", validate.Invalid(fmt.Sprintf("File %q needs a sha256 of 64 hexadecimal characters.", filePath))
	}
	for _, r := range hash {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return "", validate.Invalid(fmt.Sprintf("File %q needs a sha256 of 64 hexadecimal characters.", filePath))
		}
	}
	return hash, nil
}

func normaliseContentType(filePath, raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return guessContentType(filePath), nil
	}
	if len(value) > maxContentTypeLength {
		return "", validate.Invalid(fmt.Sprintf("Content type of %q is too long.", filePath))
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return "", validate.Invalid(fmt.Sprintf("Content type of %q has a control character.", filePath))
		}
	}
	if _, _, err := mime.ParseMediaType(value); err != nil {
		return "", validate.Invalid(fmt.Sprintf("Content type of %q is not a valid media type.", filePath))
	}
	return value, nil
}

func guessContentType(filePath string) string {
	if guessed := mime.TypeByExtension(path.Ext(filePath)); guessed != "" {
		return guessed
	}
	return fallbackContentType
}
