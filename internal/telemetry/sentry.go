package telemetry

import (
	"fmt"
	"time"

	"github.com/getsentry/sentry-go"
)

const sentryFlushTimeout = 2 * time.Second

func InitSentry(dsn, release string) (flush func(), enabled bool, err error) {
	if dsn == "" {
		return func() {}, false, nil
	}
	if err := sentry.Init(sentry.ClientOptions{Dsn: dsn, Release: release}); err != nil {
		return nil, false, fmt.Errorf("initialise sentry: %w", err)
	}
	return func() { sentry.Flush(sentryFlushTimeout) }, true, nil
}
