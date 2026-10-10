package ratelimit

import (
	"fmt"
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

func TestEvictionRemovesExpiredEntriesBeforeAnyLiveOne(t *testing.T) {
	c := &clock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	l := New(5, time.Minute, c.Now)
	l.maxKeys = 4
	l.Allow("old-1")
	l.Allow("old-2")
	c.now = c.now.Add(40 * time.Second)
	l.Allow("live-1")
	l.Allow("live-2")
	c.now = c.now.Add(30 * time.Second)
	l.Allow("fresh")
	for _, key := range []string{"live-1", "live-2", "fresh"} {
		if l.Count(key) != 1 {
			t.Errorf("live key %q was evicted while expired ones existed", key)
		}
	}
	if len(l.windows) > 4 {
		t.Errorf("tracked %d keys with a capacity of 4", len(l.windows))
	}
}

func TestEvictionRemovesTheOldestLiveWindowsFirst(t *testing.T) {
	c := &clock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	l := New(5, time.Hour, c.Now)
	l.maxKeys = 40
	for i := range 40 {
		l.Allow(fmt.Sprintf("key-%02d", i))
		c.now = c.now.Add(time.Second)
	}
	l.Allow("newcomer")
	if got := len(l.windows); got != 39 {
		t.Fatalf("tracked %d keys after eviction, want 39 (evicted 2, added 1)", got)
	}
	for _, evicted := range []string{"key-00", "key-01"} {
		if l.Count(evicted) != 0 {
			t.Errorf("%s survived although it was the oldest", evicted)
		}
	}
	for _, kept := range []string{"key-02", "key-20", "key-39", "newcomer"} {
		if l.Count(kept) != 1 {
			t.Errorf("%s was evicted although it was newer", kept)
		}
	}
}

func TestFloodingNewKeysOnlyDisplacesTheOldestCounters(t *testing.T) {
	c := &clock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	l := New(2, time.Hour, c.Now)
	l.maxKeys = 100
	l.Allow("early")
	for i := range 1000 {
		c.now = c.now.Add(time.Millisecond)
		l.Allow(fmt.Sprintf("flood-%d", i))
	}
	if len(l.windows) > 100 {
		t.Fatalf("tracked %d keys with a capacity of 100", len(l.windows))
	}
	if l.Count("early") != 0 {
		t.Error("the oldest key survived the flood")
	}
	for _, recent := range []string{"flood-999", "flood-990", "flood-960"} {
		if l.Count(recent) != 1 {
			t.Errorf("recent key %s lost its counter to the flood", recent)
		}
	}
}
