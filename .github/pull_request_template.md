## What changed and why

## How you checked it

- [ ] `gofmt -l .`, `go vet ./...` and `golangci-lint run` are clean
- [ ] `go test -race ./...` passes on SQLite, and on PostgreSQL with `TEST_DATABASE_URL` when the change touches the database
- [ ] `pnpm --dir web lint`, `typecheck`, `test` and `build` pass when the change touches the dashboard
- [ ] `docs/openapi.yaml` and `docs/agent-guide.md` match the change, and the generated API types are current
- [ ] The README or the guides in `docs/` describe the new behaviour
- [ ] The change adds no code comments and no Docker Compose file
- [ ] `CHANGELOG.md` has an entry under Unreleased when users will notice the change
