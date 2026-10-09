package mail

import (
	"errors"
	"testing"
)

func TestUnconfiguredRefusesToSend(t *testing.T) {
	var sender Sender = Unconfigured{}
	if sender.Configured() {
		t.Error("an unconfigured sender reports itself as configured")
	}
	if err := sender.Send(t.Context(), Message{To: "alex@example.com"}); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("Send error = %v, want ErrNotConfigured", err)
	}
}

func TestRecorderKeepsMessagesInOrder(t *testing.T) {
	rec := NewRecorder()
	if !rec.Configured() {
		t.Fatal("a recorder must report itself as configured")
	}
	for _, to := range []string{"alex@example.com", "sam@example.com"} {
		if err := rec.Send(t.Context(), Message{To: to, Subject: "Hi", Text: "Hello"}); err != nil {
			t.Fatal(err)
		}
	}
	got := rec.Messages()
	if len(got) != 2 || got[0].To != "alex@example.com" || got[1].To != "sam@example.com" {
		t.Fatalf("messages = %+v", got)
	}
	got[0].To = "changed@example.com"
	if rec.Messages()[0].To != "alex@example.com" {
		t.Error("Messages returned the internal slice")
	}
	rec.Reset()
	if len(rec.Messages()) != 0 {
		t.Error("Reset kept messages")
	}
}

func TestRecorderCanFail(t *testing.T) {
	rec := NewRecorder()
	boom := errors.New("boom")
	rec.FailWith(boom)
	if err := rec.Send(t.Context(), Message{}); !errors.Is(err, boom) {
		t.Fatalf("Send error = %v", err)
	}
	if len(rec.Messages()) != 0 {
		t.Error("a failed send was recorded")
	}
}
