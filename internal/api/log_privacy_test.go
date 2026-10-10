package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

func TestServerErrorsLogTheRoutePatternNotTheInviteToken(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{logToBuf: true})
		h.completeSetup()
		if _, err := db.ExecContext(t.Context(), "ALTER TABLE invites RENAME TO invites_unavailable"); err != nil {
			t.Fatal(err)
		}
		token := strings.Repeat("Qx7", 14) + "A"
		rec := h.do("GET", "/api/v1/invites/"+token+"?ref=secret-query", nil)
		expectStatus(t, rec, http.StatusInternalServerError)
		logged := h.logs.String()
		if !strings.Contains(logged, "request failed") {
			t.Fatalf("the failure was not logged: %q", logged)
		}
		if strings.Contains(logged, token) || strings.Contains(logged, "secret-query") {
			t.Errorf("the log holds the token or query: %q", logged)
		}
		if !strings.Contains(logged, "/api/v1/invites/{token}") {
			t.Errorf("the log lacks the route pattern: %q", logged)
		}
	})
}
