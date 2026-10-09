package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

type Params struct {
	MemoryKiB uint32
	Time      uint32
	Threads   uint8
	KeyLen    uint32
	SaltLen   uint32
}

var DefaultParams = Params{MemoryKiB: 64 * 1024, Time: 3, Threads: 2, KeyLen: 32, SaltLen: 16}

const (
	maxMemoryKiB = 1 << 20
	maxTime      = 12
	maxThreads   = 16
	minSaltLen   = 8
	maxSaltLen   = 64
	minKeyLen    = 16
	maxKeyLen    = 64
)

var errMalformedHash = errors.New("password hash is malformed")

func HashPassword(password string, p Params) (string, error) {
	salt := make([]byte, p.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, p.Time, p.MemoryKiB, p.Threads, p.KeyLen)
	encoding := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.MemoryKiB, p.Time, p.Threads, encoding.EncodeToString(salt), encoding.EncodeToString(key)), nil
}

func VerifyPassword(password, encoded string) (bool, error) {
	p, salt, want, err := decodeHash(encoded)
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(password), salt, p.Time, p.MemoryKiB, p.Threads, p.KeyLen)
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func NeedsRehash(encoded string, target Params) bool {
	p, _, _, err := decodeHash(encoded)
	if err != nil {
		return true
	}
	return p != target
}

func decodeHash(encoded string) (Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return Params{}, nil, nil, errMalformedHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return Params{}, nil, nil, errMalformedHash
	}
	var p Params
	var threads uint32
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.MemoryKiB, &p.Time, &threads); err != nil {
		return Params{}, nil, nil, errMalformedHash
	}
	if threads == 0 || threads > maxThreads {
		return Params{}, nil, nil, errMalformedHash
	}
	p.Threads = uint8(threads)
	encoding := base64.RawStdEncoding
	salt, err := encoding.DecodeString(parts[4])
	if err != nil {
		return Params{}, nil, nil, errMalformedHash
	}
	key, err := encoding.DecodeString(parts[5])
	if err != nil {
		return Params{}, nil, nil, errMalformedHash
	}
	p.SaltLen = uint32(len(salt))
	p.KeyLen = uint32(len(key))
	if !withinBounds(p) {
		return Params{}, nil, nil, errMalformedHash
	}
	return p, salt, key, nil
}

func withinBounds(p Params) bool {
	return p.MemoryKiB >= 8 && p.MemoryKiB <= maxMemoryKiB &&
		p.Time >= 1 && p.Time <= maxTime &&
		p.SaltLen >= minSaltLen && p.SaltLen <= maxSaltLen &&
		p.KeyLen >= minKeyLen && p.KeyLen <= maxKeyLen
}
