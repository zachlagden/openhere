package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"time"
)

const (
	DefaultResendEndpoint = "https://api.resend.com/emails"
	maxResendReply        = 4 << 10
)

var resendErrorName = regexp.MustCompile(`^[a-z_]{1,40}$`)

type resendSender struct {
	logger   *slog.Logger
	endpoint string
	client   *http.Client
}

type resendPayload struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Text    string   `json:"text"`
}

func newResendSender(logger *slog.Logger, endpoint string, timeout time.Duration) resendSender {
	return resendSender{logger: logger, endpoint: endpoint, client: &http.Client{Timeout: timeout}}
}

func (r resendSender) send(ctx context.Context, cfg Config, msg Message) error {
	if err := checkMessage(msg); err != nil {
		return err
	}
	payload, err := json.Marshal(resendPayload{From: fromHeader(cfg), To: []string{msg.To}, Subject: msg.Subject, Text: msg.Text})
	if err != nil {
		return fmt.Errorf("encode resend request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.ResendAPIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return resendTransportError(err)
	}
	defer r.closeBody(resp)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return resendStatusError(resp)
}

func resendTransportError(err error) *SendError {
	if isTimeout(err) {
		return sendError("the connection to Resend timed out", err)
	}
	return sendError("could not reach Resend", err)
}

func resendStatusError(resp *http.Response) *SendError {
	reason := fmt.Sprintf("Resend rejected the request (status %d", resp.StatusCode)
	var body struct {
		Name string `json:"name"`
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResendReply))
	if err == nil && json.Unmarshal(raw, &body) == nil && resendErrorName.MatchString(body.Name) {
		reason += ", " + body.Name
	}
	return sendError(reason+")", fmt.Errorf("resend returned status %d", resp.StatusCode))
}

func (r resendSender) closeBody(resp *http.Response) {
	if err := resp.Body.Close(); err != nil {
		r.logger.Debug("close resend response", "error", err)
	}
}
