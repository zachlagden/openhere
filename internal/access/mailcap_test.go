package access

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

func (e *env) capMails(limit *int) {
	e.t.Helper()
	e.svc = New(e.db, Options{
		Mailer: e.mailer, Params: fastParams, Now: e.clock.Now, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Spawn: func(task func()) { task() }, Secrets: e.box,
		MaxCodeMailsPerHour: func(context.Context) (int, error) { return *limit, nil },
	})
}

func TestInstanceCapStopsCodeEmailsSilently(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		limit := 2
		e.capMails(&limit)
		e.restrictWith([]string{"a@example.com", "b@example.com", "c@example.com"}, nil)
		for _, address := range []string{"a@example.com", "b@example.com", "c@example.com"} {
			e.request(address)
		}
		if got := len(e.mailer.Messages()); got != 2 {
			t.Errorf("sent %d emails under a cap of 2", got)
		}
		var stored int
		if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM viewer_codes WHERE email = 'c@example.com'").Scan(&stored); err != nil || stored != 0 {
			t.Errorf("a capped request stored %d codes, err %v", stored, err)
		}
	})
}

func TestCapIgnoresRequestsThatSendNoEmail(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		limit := 1
		e.capMails(&limit)
		e.restrictWith([]string{"sam@example.com"}, nil)
		for range 5 {
			e.request("stranger@example.com")
			e.request("not an address")
		}
		e.request("sam@example.com")
		if got := len(e.mailer.Messages()); got != 1 {
			t.Errorf("sent %d emails, want the one allowed request to still go out", got)
		}
	})
}

func TestCapZeroTurnsTheCapOffAndTheWindowResets(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		limit := 0
		e.capMails(&limit)
		e.restrictWith([]string{"sam@example.com"}, nil)
		for range 5 {
			e.request("sam@example.com")
		}
		if got := len(e.mailer.Messages()); got != 5 {
			t.Fatalf("sent %d emails with the cap off, want 5", got)
		}
		limit = 2
		for range 3 {
			e.request("sam@example.com")
		}
		if got := len(e.mailer.Messages()); got != 7 {
			t.Errorf("sent %d emails after setting a cap of 2, want 7", got)
		}
		e.clock.Advance(time.Hour + time.Second)
		e.request("sam@example.com")
		if got := len(e.mailer.Messages()); got != 8 {
			t.Errorf("sent %d emails after the window, want 8", got)
		}
	})
}
