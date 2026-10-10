package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/httptimeouts"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

func steadyUpload(t *testing.T, url, key, content string) (*http.Response, error) {
	t.Helper()
	reader, writer := io.Pipe()
	go func() {
		defer func() { _ = writer.Close() }()
		for i := 0; i < len(content); i += 20 {
			time.Sleep(100 * time.Millisecond)
			if _, err := io.WriteString(writer, content[i:i+20]); err != nil {
				return
			}
		}
	}()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, url, reader)
	if err != nil {
		t.Fatal(err)
	}
	req.ContentLength = int64(len(content))
	req.Header.Set("Authorization", "Bearer "+key)
	return http.DefaultClient.Do(req)
}

func TestSlowSteadyUploadsGetAnExtendedDeadline(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		timeouts := httptimeouts.Timeouts{
			ReadHeader: time.Second, Read: 150 * time.Millisecond, Write: 150 * time.Millisecond, Idle: time.Second,
			Base: 5 * time.Second, Rate: 1000, Cap: 10 * time.Second,
		}
		h := newHarness(t, db, harnessOptions{timeouts: timeouts})
		alex := h.rootActor()
		alex.createWorkspace("Acme Ltd", "acme")
		content := strings.Repeat("a", 100)
		pending := alex.publishNewSite("slow", siteFiles{"big.bin": content})

		srv := httptest.NewUnstartedServer(h.handler)
		timeouts.Apply(srv.Config)
		srv.Start()
		t.Cleanup(srv.Close)

		resp, err := steadyUpload(t, srv.URL+pending.Uploads[0].URL, alex.key, content)
		if err != nil {
			t.Fatalf("upload: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("status %d: %s", resp.StatusCode, body)
		}
	})
}
