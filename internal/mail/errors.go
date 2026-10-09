package mail

import (
	"context"
	"errors"
	"fmt"
	"net"
)

type SendError struct {
	Reason string
	cause  error
}

func (e *SendError) Error() string { return e.Reason }

func (e *SendError) Unwrap() error { return e.cause }

func sendError(reason string, cause error) *SendError {
	return &SendError{Reason: reason, cause: cause}
}

func Reason(err error) string {
	var sendErr *SendError
	if errors.As(err, &sendErr) {
		return sendErr.Reason
	}
	return "the email could not be sent"
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func stageError(stage string, err error) *SendError {
	if isTimeout(err) {
		return sendError(fmt.Sprintf("the connection timed out while %s", stage), err)
	}
	return sendError(fmt.Sprintf("the connection failed while %s", stage), err)
}

func replyError(what string, err error) *SendError {
	if reply, ok := asReply(err); ok {
		return sendError(fmt.Sprintf("the server rejected %s (code %d)", what, reply.Code), err)
	}
	return stageError("sending "+what, err)
}
