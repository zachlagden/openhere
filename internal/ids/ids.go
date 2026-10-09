package ids

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

const byteLength = 16

func New() (string, error) {
	buf := make([]byte, byteLength)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
