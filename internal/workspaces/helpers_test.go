package workspaces

import (
	"context"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/ids"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

var testNow = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

type fixture struct {
	t   *testing.T
	db  store.Store
	svc *Service
}

func newFixture(t *testing.T, db store.Store) *fixture {
	t.Helper()
	return &fixture{t: t, db: db, svc: New(db, Options{Now: func() time.Time { return testNow }})}
}

func (f *fixture) user(email string) string {
	f.t.Helper()
	id := storetest.InsertUser(f.t, f.db, email)
	if _, err := CreatePersonal(context.Background(), f.db, id, testNow); err != nil {
		f.t.Fatalf("CreatePersonal: %v", err)
	}
	return id
}

func (f *fixture) shared(ownerID, name string) Access {
	f.t.Helper()
	acc, err := f.svc.CreateShared(context.Background(), ownerID, name, "")
	if err != nil {
		f.t.Fatalf("CreateShared: %v", err)
	}
	return acc
}

func (f *fixture) folder(workspaceID, name string) string {
	f.t.Helper()
	id := f.newID()
	f.exec("INSERT INTO folders (id, workspace_id, name, name_key, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $5)",
		id, workspaceID, name, name, testNow)
	return id
}

func (f *fixture) site(workspaceID, slug, folderID string) string {
	f.t.Helper()
	id := f.newID()
	folder := nullable(folderID)
	f.exec("INSERT INTO sites (id, workspace_id, folder_id, slug, display_name, created_at, updated_at) VALUES ($1, $2, $3, $4, $4, $5, $5)",
		id, workspaceID, folder, slug, testNow)
	return id
}

func (f *fixture) newID() string {
	f.t.Helper()
	id, err := ids.New()
	if err != nil {
		f.t.Fatalf("ids.New: %v", err)
	}
	return id
}

func (f *fixture) exec(query string, args ...any) {
	f.t.Helper()
	if _, err := f.db.ExecContext(context.Background(), query, args...); err != nil {
		f.t.Fatalf("exec %q: %v", query, err)
	}
}

func (f *fixture) addMember(actorID, ref, email, role, scopeType, scopeID string) Member {
	f.t.Helper()
	m, err := f.svc.AddMember(context.Background(), actorID, ref, AddMemberInput{Email: email, Role: role, ScopeType: scopeType, ScopeID: scopeID})
	if err != nil {
		f.t.Fatalf("AddMember(%s, %s, %s): %v", email, role, scopeType, err)
	}
	return m
}
