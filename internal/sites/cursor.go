package sites

import (
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"github.com/zachlagden/openhere/internal/validate"
)

type cursor struct {
	createdAt time.Time
	id        string
}

func encodeCursor(site Site) string {
	raw := strconv.FormatInt(site.CreatedAt.Unix(), 10) + "." + site.ID
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(encoded string) (cursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return cursor{}, validate.Invalid("Cursor is not valid.")
	}
	seconds, id, found := strings.Cut(string(raw), ".")
	unix, parseErr := strconv.ParseInt(seconds, 10, 64)
	if !found || parseErr != nil || id == "" {
		return cursor{}, validate.Invalid("Cursor is not valid.")
	}
	return cursor{createdAt: time.Unix(unix, 0).UTC(), id: id}, nil
}
