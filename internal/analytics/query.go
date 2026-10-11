package analytics

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/zachlagden/openhere/internal/store"
)

const notRolledUp = `NOT EXISTS (SELECT 1 FROM analytics_daily d WHERE d.site_id = e.site_id AND d.day = e.day AND d.dimension = 'events')`

type reader struct {
	db store.Querier
	sc scope
	p  Params
}

func (r reader) rangeArgs(b *builder) string {
	return "day >= " + b.arg(r.p.From.Format(DayLayout)) + " AND day <= " + b.arg(r.p.To.Format(DayLayout))
}

func (r reader) rawWhere(b *builder, condition string) string {
	return r.sc(b, "e.site_id") + " AND " + r.rangeArgs(b) + " AND " + condition + " AND " + notRolledUp
}

func (r reader) rolledWhere(b *builder, name string) string {
	return r.sc(b, "site_id") + " AND " + r.rangeArgs(b) + " AND dimension = " + b.arg(name)
}

type counts struct {
	views   int64
	uniques int64
}

func (r reader) dailyTotals(ctx context.Context) (map[string]counts, error) {
	totals := map[string]counts{}
	rawB := &builder{}
	rawQuery := "SELECT day, COUNT(*), COUNT(DISTINCT visitor) FROM analytics_events e WHERE " +
		r.rawWhere(rawB, humanViews) + " GROUP BY day"
	if err := r.collect(ctx, rawQuery, rawB.args, func(key string, c counts) { add(totals, key, c) }); err != nil {
		return nil, err
	}
	rolledB := &builder{}
	rolledQuery := "SELECT day, SUM(views), SUM(uniques) FROM analytics_daily WHERE " +
		r.rolledWhere(rolledB, dimTotal) + " GROUP BY day"
	if err := r.collect(ctx, rolledQuery, rolledB.args, func(key string, c counts) { add(totals, key, c) }); err != nil {
		return nil, err
	}
	return totals, nil
}

func (r reader) hourlyTotals(ctx context.Context) (map[string]counts, error) {
	b := &builder{}
	query := "SELECT day, hour, COUNT(*), COUNT(DISTINCT visitor) FROM analytics_events e WHERE " +
		r.sc(b, "e.site_id") + " AND " + r.rangeArgs(b) + " AND " + humanViews + " GROUP BY day, hour"
	rows, err := r.db.QueryContext(ctx, query, b.args...)
	if err != nil {
		return nil, fmt.Errorf("query hourly analytics: %w", err)
	}
	totals := map[string]counts{}
	for rows.Next() {
		var day string
		var hour int
		var c counts
		if err := rows.Scan(&day, &hour, &c.views, &c.uniques); err != nil {
			return nil, errors.Join(fmt.Errorf("scan hourly analytics: %w", err), rows.Close())
		}
		add(totals, fmt.Sprintf("%s %02d", day, hour), c)
	}
	return totals, closeRows(rows, "hourly analytics")
}

func (r reader) ranked(ctx context.Context, dim dimension) (map[string]counts, error) {
	merged := map[string]counts{}
	rawB := &builder{}
	rawQuery := "SELECT " + dim.column + ", COUNT(*), COUNT(DISTINCT visitor) FROM analytics_events e WHERE " +
		r.rawWhere(rawB, dim.condition()) + " GROUP BY day, " + dim.column
	if err := r.collect(ctx, rawQuery, rawB.args, func(key string, c counts) { add(merged, key, c) }); err != nil {
		return nil, err
	}
	rolledB := &builder{}
	rolledQuery := "SELECT value, SUM(views), SUM(uniques) FROM analytics_daily WHERE " +
		r.rolledWhere(rolledB, dim.name) + " GROUP BY value"
	if err := r.collect(ctx, rolledQuery, rolledB.args, func(key string, c counts) { add(merged, key, c) }); err != nil {
		return nil, err
	}
	return merged, nil
}

func (r reader) collect(ctx context.Context, query string, args []any, each func(key string, c counts)) error {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("query analytics: %w", err)
	}
	for rows.Next() {
		var key string
		var views, uniques sql.NullInt64
		if err := rows.Scan(&key, &views, &uniques); err != nil {
			return errors.Join(fmt.Errorf("scan analytics: %w", err), rows.Close())
		}
		each(key, counts{views: views.Int64, uniques: uniques.Int64})
	}
	return closeRows(rows, "analytics")
}

func closeRows(rows *sql.Rows, what string) error {
	if err := rows.Err(); err != nil {
		return errors.Join(fmt.Errorf("read %s: %w", what, err), rows.Close())
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close %s: %w", what, err)
	}
	return nil
}

func add(into map[string]counts, key string, c counts) {
	current := into[key]
	current.views += c.views
	current.uniques += c.uniques
	into[key] = current
}

func rank(merged map[string]counts, top int) []Ranked {
	list := make([]Ranked, 0, len(merged))
	for value, c := range merged {
		list = append(list, Ranked{Value: value, Views: c.views, Uniques: c.uniques})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Views != list[j].Views {
			return list[i].Views > list[j].Views
		}
		return list[i].Value < list[j].Value
	})
	if len(list) > top {
		list = list[:top]
	}
	return list
}

func series(totals map[string]counts, p Params) []Point {
	if p.Interval == IntervalHour {
		return hourlySeries(totals, p)
	}
	points := make([]Point, 0, p.days())
	for day := p.From; !day.After(p.To); day = day.AddDate(0, 0, 1) {
		c := totals[day.Format(DayLayout)]
		points = append(points, Point{Start: day, Views: c.views, Uniques: c.uniques})
	}
	return points
}

func hourlySeries(totals map[string]counts, p Params) []Point {
	points := make([]Point, 0, p.days()*24)
	for day := p.From; !day.After(p.To); day = day.AddDate(0, 0, 1) {
		for hour := range 24 {
			c := totals[fmt.Sprintf("%s %02d", day.Format(DayLayout), hour)]
			points = append(points, Point{Start: day.Add(time.Duration(hour) * time.Hour), Views: c.views, Uniques: c.uniques})
		}
	}
	return points
}
