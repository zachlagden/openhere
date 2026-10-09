package mail

import (
	"context"
	"errors"
	"sync"
)

var ErrNotConfigured = errors.New("no mail sender is configured")

type Message struct {
	To      string
	Subject string
	Text    string
}

type Sender interface {
	Send(ctx context.Context, msg Message) error
	Configured() bool
}

type Unconfigured struct{}

func (Unconfigured) Send(context.Context, Message) error { return ErrNotConfigured }

func (Unconfigured) Configured() bool { return false }

type Recorder struct {
	mu       sync.Mutex
	messages []Message
	failure  error
}

func NewRecorder() *Recorder {
	return &Recorder{}
}

func (r *Recorder) Send(_ context.Context, msg Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failure != nil {
		return r.failure
	}
	r.messages = append(r.messages, msg)
	return nil
}

func (r *Recorder) Configured() bool { return true }

func (r *Recorder) FailWith(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failure = err
}

func (r *Recorder) Messages() []Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Message(nil), r.messages...)
}

func (r *Recorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messages = nil
}
