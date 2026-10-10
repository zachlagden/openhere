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

func TestRecordCountsAndFullReportsWithoutConsuming(t *testing.T) {
	c := &clock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	l := New(2, time.Minute, c.Now)
	if full, _ := l.Full("a"); full || l.Count("a") != 0 {
		t.Fatal("an unseen key reported failures")
	}
	l.Record("a")
	if full, _ := l.Full("a"); full || l.Count("a") != 1 {
		t.Errorf("after one record: full %v, count %d", full, l.Count("a"))
	}
	for range 3 {
		l.Full("a")
	}
	if l.Count("a") != 1 {
		t.Error("Full consumed an attempt")
	}
	l.Record("a")
	full, wait := l.Full("a")
	if !full || wait != time.Minute {
		t.Errorf("after two records: full %v, wait %v", full, wait)
	}
	c.now = c.now.Add(time.Minute)
	if full, _ := l.Full("a"); full || l.Count("a") != 0 {
		t.Error("the counter survived its window")
	}
}
