package mail

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"net/textproto"
	"strconv"
	"time"

	"github.com/zachlagden/selfhost/internal/settings"
)

type smtpSender struct {
	logger    *slog.Logger
	tlsConfig *tls.Config
	timeout   time.Duration
	now       func() time.Time
}

func (s smtpSender) send(ctx context.Context, cfg Config, msg Message) error {
	body, err := buildMIME(cfg, msg, s.now())
	if err != nil {
		return err
	}
	if err := requireEncryptedAuth(cfg.SMTP); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	client, err := s.connect(ctx, cfg.SMTP)
	if err != nil {
		return err
	}
	defer s.close(client)
	if err := s.authenticate(client, cfg.SMTP); err != nil {
		return err
	}
	return s.deliver(client, cfg.FromAddress, msg.To, body)
}

func (s smtpSender) close(client *smtp.Client) {
	if err := client.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		s.logger.Debug("close smtp connection", "error", err)
	}
}

func (s smtpSender) closeConn(conn net.Conn) {
	if err := conn.Close(); err != nil {
		s.logger.Debug("close smtp connection", "error", err)
	}
}

func (s smtpSender) tlsFor(host string) *tls.Config {
	base := s.tlsConfig
	if base == nil {
		base = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	clone := base.Clone()
	clone.ServerName = host
	return clone
}

func (s smtpSender) connect(ctx context.Context, cfg SMTPConfig) (*smtp.Client, error) {
	address := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, stageError("connecting to the server", err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			s.closeConn(conn)
			return nil, stageError("connecting to the server", err)
		}
	}
	if cfg.Security == settings.SecurityTLS {
		tlsConn := tls.Client(conn, s.tlsFor(cfg.Host))
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			s.closeConn(conn)
			return nil, tlsError(err)
		}
		conn = tlsConn
	}
	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		s.closeConn(conn)
		return nil, replyOrStage("reading the server greeting", "the connection", err)
	}
	if cfg.Security == settings.SecurityStartTLS {
		if err := s.startTLS(client, cfg.Host); err != nil {
			s.close(client)
			return nil, err
		}
	}
	return client, nil
}

func (s smtpSender) startTLS(client *smtp.Client, host string) error {
	if ok, _ := client.Extension("STARTTLS"); !ok {
		return sendError("the server does not offer STARTTLS; change the security setting", nil)
	}
	if err := client.StartTLS(s.tlsFor(host)); err != nil {
		return tlsError(err)
	}
	return nil
}

func (s smtpSender) authenticate(client *smtp.Client, cfg SMTPConfig) error {
	if cfg.Username == "" {
		return nil
	}
	ok, advertised := client.Extension("AUTH")
	if !ok {
		return sendError("the server does not offer authentication", nil)
	}
	auth, found := chooseAuth(advertised, cfg.Username, cfg.Password)
	if !found {
		return sendError("the server offers no supported authentication method", nil)
	}
	if err := client.Auth(auth); err != nil {
		return authError(err)
	}
	return nil
}

func (s smtpSender) deliver(client *smtp.Client, from, to string, body []byte) error {
	if err := client.Mail(from); err != nil {
		return replyError("the sender address", err)
	}
	if err := client.Rcpt(to); err != nil {
		return replyError("the recipient address", err)
	}
	writer, err := client.Data()
	if err != nil {
		return replyError("the message", err)
	}
	if _, err := writer.Write(body); err != nil {
		return stageError("sending the message", err)
	}
	if err := writer.Close(); err != nil {
		return replyError("the message", err)
	}
	if err := client.Quit(); err != nil {
		s.logger.Debug("end smtp session", "error", err)
	}
	return nil
}

func tlsError(err error) *SendError {
	if isTimeout(err) {
		return sendError("the connection timed out during the TLS handshake", err)
	}
	return sendError("the TLS handshake failed; check the security setting and the server certificate", err)
}

func authError(err error) *SendError {
	if isTimeout(err) {
		return sendError("the connection timed out while signing in", err)
	}
	if reply, ok := asReply(err); ok {
		switch reply.Code {
		case 530, 534, 535, 454:
			return sendError("the server rejected the username or password", err)
		}
		return sendError(fmt.Sprintf("signing in failed (code %d)", reply.Code), err)
	}
	return sendError("signing in failed", err)
}

func asReply(err error) (*textproto.Error, bool) {
	var reply *textproto.Error
	return reply, errors.As(err, &reply)
}

func replyOrStage(stage, what string, err error) *SendError {
	if reply, ok := asReply(err); ok {
		return sendError(fmt.Sprintf("the server refused %s (code %d)", what, reply.Code), err)
	}
	return stageError(stage, err)
}
