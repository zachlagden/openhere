package analytics

import (
	"context"
	"net/url"
	"time"

	"github.com/zachlagden/openhere/internal/settings"
	"github.com/zachlagden/openhere/internal/sites"
	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/workspaces"
)

type Availability interface {
	Available() bool
}

type Options struct {
	Now      func() time.Time
	Recorder *Recorder
	Location Availability
}

type Service struct {
	db       store.Store
	sites    *sites.Service
	ws       *workspaces.Service
	settings *settings.Service
	recorder *Recorder
	location Availability
	now      func() time.Time
}

func New(db store.Store, siteSvc *sites.Service, ws *workspaces.Service, settingsSvc *settings.Service, opts Options) *Service {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Service{db: db, sites: siteSvc, ws: ws, settings: settingsSvc, recorder: opts.Recorder, location: opts.Location, now: now}
}

func (s *Service) ParseParams(query url.Values) (Params, error) {
	return ParseParams(query, s.clock())
}

func (s *Service) SiteReport(ctx context.Context, userID, siteRef string, p Params) (Report, error) {
	site, err := s.sites.Authorize(ctx, userID, siteRef, workspaces.RoleRead)
	if err != nil {
		return Report{}, err
	}
	return s.report(ctx, siteScope(site.ID), p)
}

func (s *Service) WorkspaceReport(ctx context.Context, userID, workspaceRef string, p Params) (Report, error) {
	acc, err := s.ws.Authorize(ctx, userID, workspaceRef, workspaces.RoleRead)
	if err != nil {
		return Report{}, err
	}
	if acc.ScopedOnly() {
		return Report{}, workspaces.ErrForbidden
	}
	return s.report(ctx, workspaceScope(userID, acc.Workspace.ID), p)
}

func (s *Service) InstanceReport(ctx context.Context, p Params) (Report, error) {
	report, err := s.report(ctx, instanceScope(), p)
	if err != nil {
		return Report{}, err
	}
	if s.recorder != nil {
		dropped := s.recorder.Dropped()
		report.DroppedEvents = &dropped
	}
	return report, nil
}

func (s *Service) report(ctx context.Context, sc scope, p Params) (Report, error) {
	r := reader{db: s.db, sc: sc, p: p}
	daily, err := r.dailyTotals(ctx)
	if err != nil {
		return Report{}, err
	}
	report := Report{From: p.From, To: p.To, Interval: p.Interval, GeneratedAt: s.clock().Truncate(time.Second)}
	for _, c := range daily {
		report.Totals.Views += c.views
		report.Totals.Uniques += c.uniques
	}
	points := daily
	if p.Interval == IntervalHour {
		if points, err = r.hourlyTotals(ctx); err != nil {
			return Report{}, err
		}
	}
	report.Series = series(points, p)
	if err := s.fillRanked(ctx, r, &report); err != nil {
		return Report{}, err
	}
	report.LocationAvailable = s.location != nil && s.location.Available()
	return report, nil
}

func (s *Service) fillRanked(ctx context.Context, r reader, report *Report) error {
	targets := map[string]*[]Ranked{
		dimPath: &report.Paths, dimReferrer: &report.Referrers, dimCountry: &report.Countries, dimCity: &report.Cities,
		dimBrowser: &report.Browsers, dimOS: &report.Systems, dimDevice: &report.Devices, dimStatus: &report.Statuses,
	}
	for _, dim := range dimensions {
		target, ok := targets[dim.name]
		if !ok {
			continue
		}
		merged, err := r.ranked(ctx, dim)
		if err != nil {
			return err
		}
		if dim.name == dimDevice {
			report.Totals.BotViews = merged[DeviceBot].views
		}
		*target = rank(merged, r.p.Top)
	}
	return nil
}
