package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

const (
	KeyBytes   = 32
	sealPrefix = "v1:"
)

var (
	ErrMalformed = errors.New("the sealed value is malformed")
	ErrOpen      = errors.New("the sealed value cannot be opened with this key")
)

type Box struct {
	aead cipher.AEAD
	key  []byte
}

func New(key []byte) (*Box, error) {
	if len(key) != KeyBytes {
		return nil, fmt.Errorf("the secret key must be %d bytes, got %d", KeyBytes, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	return &Box{aead: aead, key: append([]byte(nil), key...)}, nil
}

func (b *Box) Seal(plaintext string) (string, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	sealed := b.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return sealPrefix + base64.RawStdEncoding.EncodeToString(sealed), nil
}

func (b *Box) Open(sealed string) (string, error) {
	encoded, found := strings.CutPrefix(sealed, sealPrefix)
	if !found {
		return "", ErrMalformed
	}
	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil || len(raw) < b.aead.NonceSize()+b.aead.Overhead() {
		return "", ErrMalformed
	}
	nonce, body := raw[:b.aead.NonceSize()], raw[b.aead.NonceSize():]
	plain, err := b.aead.Open(nil, nonce, body, nil)
	if err != nil {
		return "", ErrOpen
	}
	return string(plain), nil
}

func (b *Box) Sign(purpose, message string) []byte {
	derive := hmac.New(sha256.New, b.key)
	derive.Write([]byte("selfhost-derive|" + purpose))
	mac := hmac.New(sha256.New, derive.Sum(nil))
	mac.Write([]byte(message))
	return mac.Sum(nil)
}
