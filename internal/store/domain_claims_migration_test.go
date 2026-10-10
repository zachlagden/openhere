package store

import (
	"context"
	"testing"
	"time"
)

func TestMigrationKeepsDomainsAndAllowsCompetingUnverifiedClaims(t *testing.T) {
	for _, target := range testTargets() {
		t.Run(target.name, func(t *testing.T) {
			ctx := context.Background()
			s := openMigratedTo(t, target, 20)
			seedSite(t, s, siteID, "first")
			stamp := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
			insert := func(id, site, domain string, verified bool) error {
				var verifiedAt any
				if verified {
					verifiedAt = stamp
				}
				_, err := s.ExecContext(ctx,
					`INSERT INTO site_domains (id, site_id, domain, challenge_token, verified_at, created_at, claim_started_at)
					VALUES ($1, $2, $3, 'token', $4, $5, $5)`, id, site, domain, verifiedAt, stamp)
				return err
			}
			if err := insert("d1", siteID, "www.example.org", true); err != nil {
				t.Fatalf("insert before migration: %v", err)
			}
			if err := s.Migrate(ctx); err != nil {
				t.Fatalf("Migrate: %v", err)
			}
			var kept int
			if err := s.QueryRowContext(ctx, "SELECT COUNT(*) FROM site_domains WHERE id = 'd1' AND verified_at IS NOT NULL").Scan(&kept); err != nil || kept != 1 {
				t.Fatalf("existing domain rows = %d, err %v", kept, err)
			}
			seedSite(t, s, otherSite, "second")
			if err := insert("d2", otherSite, "www.example.org", false); err != nil {
				t.Errorf("a competing unverified claim was refused: %v", err)
			}
			if err := insert("d3", otherSite, "www.example.org", true); err == nil {
				t.Error("two verified claims of one name were accepted")
			}
		})
	}
}
