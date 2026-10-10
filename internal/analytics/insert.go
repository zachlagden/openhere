package analytics

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/zachlagden/selfhost/internal/store"
)

const (
	eventColumns    = "site_id, occurred_at, day, hour, path, status_class, referrer_host, country, city, browser, os, device, visitor"
	eventColumnsLen = 13
)

func eventArgs(e Event) []any {
	var visitor sql.NullString
	if e.Visitor != "" {
		visitor = sql.NullString{String: e.Visitor, Valid: true}
	}
	return []any{
		e.SiteID, e.OccurredAt.UTC().Truncate(time.Second), e.Day(), e.Hour(), e.Path, e.StatusClass,
		e.ReferrerHost, e.Country, e.City, e.Browser, e.OS, e.Device, visitor,
	}
}

func insertEvents(ctx context.Context, q store.Querier, events []Event) error {
	if len(events) == 0 {
		return nil
	}
	var query strings.Builder
	query.WriteString("INSERT INTO analytics_events (" + eventColumns + ") VALUES ")
	args := make([]any, 0, len(events)*eventColumnsLen)
	for i, e := range events {
		if i > 0 {
			query.WriteString(", ")
		}
		query.WriteString("(")
		for col := range eventColumnsLen {
			if col > 0 {
				query.WriteString(", ")
			}
			query.WriteString("$" + strconv.Itoa(i*eventColumnsLen+col+1))
		}
		query.WriteString(")")
		args = append(args, eventArgs(e)...)
	}
	if _, err := q.ExecContext(ctx, query.String(), args...); err != nil {
		return fmt.Errorf("insert analytics events: %w", err)
	}
	return nil
}
