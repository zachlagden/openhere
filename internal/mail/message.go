package mail

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"time"
)

func fromHeader(cfg Config) string {
	return (&mail.Address{Name: cfg.FromName, Address: cfg.FromAddress}).String()
}

func checkMessage(msg Message) error {
	parsed, err := mail.ParseAddress(msg.To)
	if err != nil || parsed.Address != msg.To {
		return sendError("the recipient address is not valid", errors.New("invalid recipient"))
	}
	if strings.ContainsAny(msg.Subject, "\r\n") {
		return sendError("the subject contains a line break", errors.New("invalid subject"))
	}
	return nil
}

func buildMIME(cfg Config, msg Message, now time.Time) ([]byte, error) {
	if err := checkMessage(msg); err != nil {
		return nil, err
	}
	id := make([]byte, 12)
	if _, err := rand.Read(id); err != nil {
		return nil, fmt.Errorf("generate message id: %w", err)
	}
	domain := cfg.FromAddress[strings.LastIndex(cfg.FromAddress, "@")+1:]
	var out bytes.Buffer
	header := func(name, value string) { out.WriteString(name + ": " + value + "\r\n") }
	header("From", fromHeader(cfg))
	header("To", msg.To)
	header("Subject", mime.QEncoding.Encode("utf-8", msg.Subject))
	header("Date", now.UTC().Format(time.RFC1123Z))
	header("Message-ID", "<"+hex.EncodeToString(id)+"@"+domain+">")
	header("MIME-Version", "1.0")
	header("Content-Type", `text/plain; charset="utf-8"`)
	header("Content-Transfer-Encoding", "quoted-printable")
	out.WriteString("\r\n")
	body := quotedprintable.NewWriter(&out)
	if _, err := body.Write([]byte(strings.ReplaceAll(msg.Text, "\n", "\r\n"))); err != nil {
		return nil, fmt.Errorf("encode body: %w", err)
	}
	if err := body.Close(); err != nil {
		return nil, fmt.Errorf("encode body: %w", err)
	}
	return out.Bytes(), nil
}
