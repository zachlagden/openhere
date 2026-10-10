package httptimeouts

import (
	"errors"
	"log/slog"
	"net/http"
	"time"
)

const (
	DefaultReadHeader = 10 * time.Second
	DefaultRead       = 30 * time.Second
	DefaultWrite      = 60 * time.Second
	DefaultIdle       = 90 * time.Second
	DefaultBase       = time.Minute
	DefaultRate       = int64(2 << 20)
	DefaultCap        = 30 * time.Minute
)

type Timeouts struct {
	ReadHeader time.Duration
	Read       time.Duration
	Write      time.Duration
	Idle       time.Duration
	Base       time.Duration
	Rate       int64
	Cap        time.Duration
}

func Defaults() Timeouts {
	return Timeouts{
		ReadHeader: DefaultReadHeader, Read: DefaultRead, Write: DefaultWrite, Idle: DefaultIdle,
		Base: DefaultBase, Rate: DefaultRate, Cap: DefaultCap,
	}
}

func (t Timeouts) OrDefaults() Timeouts {
	if t == (Timeouts{}) {
		return Defaults()
	}
	return t
}

func (t Timeouts) NewServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr: addr, Handler: handler,
		ReadHeaderTimeout: t.ReadHeader, ReadTimeout: t.Read, WriteTimeout: t.Write, IdleTimeout: t.Idle,
	}
}

func (t Timeouts) Apply(srv *http.Server) {
	srv.ReadHeaderTimeout = t.ReadHeader
	srv.ReadTimeout = t.Read
	srv.WriteTimeout = t.Write
	srv.IdleTimeout = t.Idle
}

func (t Timeouts) Extension(size int64) time.Duration {
	if size < 0 {
		size = 0
	}
	extra := time.Duration(0)
	if t.Rate > 0 {
		extra = time.Duration(size/t.Rate) * time.Second
	}
	return min(t.Base+extra, t.Cap)
}

func (t Timeouts) ExtendUpload(w http.ResponseWriter, size int64) {
	deadline := time.Now().Add(t.Extension(size))
	controller := http.NewResponseController(w)
	logUnsupported("extend the read deadline", controller.SetReadDeadline(deadline))
	logUnsupported("extend the write deadline", controller.SetWriteDeadline(deadline.Add(t.Write)))
}

func (t Timeouts) ExtendDownload(w http.ResponseWriter, size int64) {
	deadline := time.Now().Add(t.Extension(size))
	logUnsupported("extend the write deadline", http.NewResponseController(w).SetWriteDeadline(deadline))
}

func logUnsupported(action string, err error) {
	if err == nil || errors.Is(err, http.ErrNotSupported) {
		return
	}
	slog.Debug(action, "error", err)
}
