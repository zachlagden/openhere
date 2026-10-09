package store

import "testing"

func TestRebindForSQLite(t *testing.T) {
	cases := map[string]struct{ in, want string }{
		"no parameters":      {"SELECT 1", "SELECT 1"},
		"one parameter":      {"SELECT value FROM instance_settings WHERE key = $1", "SELECT value FROM instance_settings WHERE key = ?1"},
		"repeated parameter": {"SELECT $1, $1, $2", "SELECT ?1, ?1, ?2"},
		"two digit":          {"VALUES ($10)", "VALUES (?10)"},
		"inside string":      {"SELECT '$1', $1", "SELECT '$1', ?1"},
		"dollar without num": {"SELECT $a", "SELECT $a"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := rebindForSQLite(tc.in); got != tc.want {
				t.Errorf("rebindForSQLite(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
