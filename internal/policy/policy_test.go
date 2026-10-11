package policy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/zachlagden/openhere/internal/auth"
	"github.com/zachlagden/openhere/internal/settings"
	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
	"github.com/zachlagden/openhere/internal/workspaces"
)

func TestSharedCreationFollowsTheSetting(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		authSvc, err := auth.New(db, auth.Options{
			KeyPrefix: "sk_", Params: auth.Params{MemoryKiB: 8, Time: 1, Threads: 1, KeyLen: 16, SaltLen: 8},
			Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		})
		if err != nil {
			t.Fatal(err)
		}
		root, err := authSvc.Setup(ctx, "alex@example.com", "correct horse battery")
		if err != nil {
			t.Fatal(err)
		}
		sam, err := authSvc.CreateUser(ctx, "sam@example.com", "correct horse battery")
		if err != nil {
			t.Fatal(err)
		}
		settingsSvc := settings.New(db, settings.Options{})
		wsSvc := workspaces.New(db, workspaces.Options{MayCreateShared: MayCreateShared(settingsSvc, authSvc)})

		if _, err := wsSvc.CreateShared(ctx, sam.ID, "Acme Ltd", "acme"); err != nil {
			t.Fatalf("default policy refused a member: %v", err)
		}
		patch := map[string]json.RawMessage{settings.KeyWhoCanCreate: json.RawMessage(`"root"`)}
		if _, err := settingsSvc.Update(ctx, patch); err != nil {
			t.Fatal(err)
		}
		if _, err := wsSvc.CreateShared(ctx, sam.ID, "Beta Ltd", "beta"); !errors.Is(err, ErrSharedCreationRestricted) {
			t.Fatalf("member error = %v, want ErrSharedCreationRestricted", err)
		}
		if _, err := wsSvc.CreateShared(ctx, root.ID, "Beta Ltd", "beta"); err != nil {
			t.Fatalf("root was refused: %v", err)
		}
		patch[settings.KeyWhoCanCreate] = json.RawMessage(`"any"`)
		if _, err := settingsSvc.Update(ctx, patch); err != nil {
			t.Fatal(err)
		}
		if _, err := wsSvc.CreateShared(ctx, sam.ID, "Gamma Ltd", "gamma"); err != nil {
			t.Fatalf("opening creation again did not take effect: %v", err)
		}
	})
}
