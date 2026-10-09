package ratelimit

import (
	"testing"
	"time"
)

type clock struct{ now time.Time }

func (c *clock) Now() time.Time { return c.now }

func TestAllowCountsWithinWindow(t *testing.T) {
	c := &clock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	l := New(3, time.Minute, c.Now)
	for i := range 3 {
		if ok, _ := l.Allow("a"); !ok {
			t.Fatalf("attempt %d was refused", i+1)
		}
	}
	ok, wait := l.Allow("a")
	if ok || wait != time.Minute {
		t.Errorf("fourth attempt = %v, %v; want refused with 1m wait", ok, wait)
	}
	if ok, _ := l.Allow("b"); !ok {
		t.Error("a different key shares the counter")
	}
}

func TestWindowExpires(t *testing.T) {
	c := &clock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	l := New(1, time.Minute, c.Now)
	l.Allow("a")
	c.now = c.now.Add(30 * time.Second)
	if ok, wait := l.Allow("a"); ok || wait != 30*time.Second {
		t.Errorf("mid-window = %v, %v; want refused with 30s wait", ok, wait)
	}
	c.now = c.now.Add(30 * time.Second)
	if ok, _ := l.Allow("a"); !ok {
		t.Error("attempt after the window was refused")
	}
}

func TestReset(t *testing.T) {
	l := New(1, time.Minute, nil)
	l.Allow("a")
	l.Reset("a")
	if ok, _ := l.Allow("a"); !ok {
		t.Error("Reset did not clear the counter")
	}
}
