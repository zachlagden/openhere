package analytics

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/zachlagden/openhere/internal/store"
)

type dayKey struct {
	siteID string
	day    string
}

func (s *Service) Rollup(ctx context.Context) error {
	today := s.clock().Format(DayLayout)
	raw, err := countRawDays(ctx, s.db, today)
	if err != nil {
		return err
	}
	rolled, err := countRolledDays(ctx, s.db, today)
	if err != nil {
		return err
	}
	var problems []error
	for key, count := range raw {
		if rolled[key] == count {
			continue
		}
		if err := rollupDay(ctx, s.db, key); err != nil {
			problems = append(problems, err)
		}
	}
	return errors.Join(problems...)
}

func countRawDays(ctx context.Context, q store.Querier, before string) (map[dayKey]int64, error) {
	rows, err := q.QueryContext(ctx,
		"SELECT site_id, day, COUNT(*) FROM analytics_events WHERE day < $1 GROUP BY site_id, day", before)
	if err != nil {
		return nil, fmt.Errorf("count raw analytics days: %w", err)
	}
	return collectCounts(rows)
}

func countRolledDays(ctx context.Context, q store.Querier, before string) (map[dayKey]int64, error) {
	rows, err := q.QueryContext(ctx,
		"SELECT site_id, day, views FROM analytics_daily WHERE dimension = $1 AND day < $2", dimEvents, before)
	if err != nil {
		return nil, fmt.Errorf("count rolled analytics days: %w", err)
	}
	return collectCounts(rows)
}

func collectCounts(rows *sql.Rows) (map[dayKey]int64, error) {
	counts := map[dayKey]int64{}
	for rows.Next() {
		var key dayKey
		var count int64
		if err := rows.Scan(&key.siteID, &key.day, &count); err != nil {
			return nil, errors.Join(fmt.Errorf("scan analytics count: %w", err), rows.Close())
		}
		counts[key] = count
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Join(fmt.Errorf("read analytics counts: %w", err), rows.Close())
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close analytics counts: %w", err)
	}
	return counts, nil
}

func rollupDay(ctx context.Context, db store.Store, key dayKey) error {
	return store.WithTx(ctx, db, func(q store.Querier) error {
		count, err := countDayEvents(ctx, q, key)
		if err != nil {
			return err
		}
		if _, err := q.ExecContext(ctx, "DELETE FROM analytics_daily WHERE site_id = $1 AND day = $2", key.siteID, key.day); err != nil {
			return fmt.Errorf("clear analytics rollup: %w", err)
		}
		for _, dim := range dimensions {
			if err := rollupDimension(ctx, q, key, dim); err != nil {
				return err
			}
		}
		return insertRolled(ctx, q, key, dimEvents, rolledValue{views: count})
	})
}

func rollupDimension(ctx context.Context, q store.Querier, key dayKey, dim dimension) error {
	column := "''"
	group := ""
	order := ""
	if dim.column != "" {
		column = dim.column
		group = " GROUP BY " + dim.column
		order = ", " + dim.column
	}
	query := "SELECT " + column + ", COUNT(*), COUNT(DISTINCT visitor) FROM analytics_events WHERE site_id = $1 AND day = $2 AND " +
		dim.condition() + group + " ORDER BY COUNT(*) DESC" + order + fmt.Sprintf(" LIMIT %d", maxRolledValues)
	rows, err := q.QueryContext(ctx, query, key.siteID, key.day)
	if err != nil {
		return fmt.Errorf("aggregate %s: %w", dim.name, err)
	}
	var found []rolledValue
	for rows.Next() {
		var v rolledValue
		if err := rows.Scan(&v.value, &v.views, &v.uniques); err != nil {
			return errors.Join(fmt.Errorf("scan %s: %w", dim.name, err), rows.Close())
		}
		found = append(found, v)
	}
	if err := rows.Err(); err != nil {
		return errors.Join(fmt.Errorf("read %s: %w", dim.name, err), rows.Close())
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close %s: %w", dim.name, err)
	}
	for _, v := range found {
		if dim.column == "" && v.views == 0 {
			continue
		}
		if err := insertRolled(ctx, q, key, dim.name, v); err != nil {
			return err
		}
	}
	return nil
}

type rolledValue struct {
	value   string
	views   int64
	uniques int64
}

func insertRolled(ctx context.Context, q store.Querier, key dayKey, name string, v rolledValue) error {
	_, err := q.ExecContext(ctx,
		"INSERT INTO analytics_daily (site_id, day, dimension, value, views, uniques) VALUES ($1, $2, $3, $4, $5, $6)",
		key.siteID, key.day, name, v.value, v.views, v.uniques)
	if err != nil {
		return fmt.Errorf("store %s rollup: %w", name, err)
	}
	return nil
}

func countDayEvents(ctx context.Context, q store.Querier, key dayKey) (int64, error) {
	var count int64
	err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM analytics_events WHERE site_id = $1 AND day = $2", key.siteID, key.day).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count day events: %w", err)
	}
	return count, nil
}

func (s *Service) clock() time.Time {
	return s.now().UTC()
}
