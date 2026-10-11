package access

import (
	"crypto/rand"
	"log/slog"

	"github.com/zachlagden/openhere/internal/secrets"
)

func (s *Service) sign(purpose, message string) []byte {
	return s.box.Sign(purpose, message)
}

func ephemeralBox(logger *slog.Logger) *secrets.Box {
	key := make([]byte, secrets.KeyBytes)
	if _, err := rand.Read(key); err != nil {
		logger.Error("generate an in-memory viewer key", "error", err)
	}
	box, err := secrets.New(key)
	if err != nil {
		panic(err)
	}
	return box
}
