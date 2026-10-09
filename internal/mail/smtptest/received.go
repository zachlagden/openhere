package smtptest

import (
	"io"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"strings"
)

func (r Received) parsed() (*mail.Message, error) {
	return mail.ReadMessage(strings.NewReader(strings.ReplaceAll(r.Data, "\n", "\r\n")))
}

func (r Received) Header(name string) string {
	msg, err := r.parsed()
	if err != nil {
		return ""
	}
	decoded, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get(name))
	if err != nil {
		return msg.Header.Get(name)
	}
	return decoded
}

func (r Received) Text() string {
	msg, err := r.parsed()
	if err != nil {
		return ""
	}
	body, err := io.ReadAll(quotedprintable.NewReader(msg.Body))
	if err != nil {
		return ""
	}
	return strings.ReplaceAll(string(body), "\r\n", "\n")
}
