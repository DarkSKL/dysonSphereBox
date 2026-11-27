package requestcache

import (
	"net/http"
	"testing"
	"time"
)

func TestFreshForHonorsResponsePolicy(t *testing.T) {
	now := time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)
	past := now.Add(-5 * time.Second).Format(http.TimeFormat)
	for _, test := range []struct {
		name   string
		header http.Header
		want   time.Duration
	}{
		{name: "method limit", want: 30 * time.Second},
		{name: "no store", header: http.Header{"Cache-Control": {"private, NO-STORE", "max-age=15"}}},
		{name: "requires validation", header: http.Header{"Cache-Control": {"no-cache, max-age=20"}}},
		{name: "private with age", header: http.Header{"Cache-Control": {"private, max-age=15"}, "Age": {"2"}, "Date": {past}}, want: 10 * time.Second},
		{name: "exceeds method limit", header: http.Header{"Cache-Control": {"private, max-age=120"}}, want: 30 * time.Second},
		{name: "expired", header: http.Header{"Cache-Control": {"max-age=3"}, "Age": {"9"}}},
		{name: "duplicate age", header: http.Header{"Cache-Control": {"max-age=5", "max-age=10"}}},
		{name: "unknown variant", header: http.Header{"Vary": {"Accept-Language"}}},
		{name: "expires header", header: http.Header{"Expires": {now.Add(12 * time.Second).Format(http.TimeFormat)}}, want: 12 * time.Second},
	} {
		t.Run(test.name, func(t *testing.T) {
			if actual := FreshFor(test.header, now, 30*time.Second); actual != test.want {
				t.Fatalf("freshness = %s; want %s", actual, test.want)
			}
		})
	}
}

func TestViewFreshForKeepsNoStoreResponseAsLocalView(t *testing.T) {
	now := time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)
	limit := 30 * time.Second
	header := http.Header{"Cache-Control": {"no-store"}}
	if got := FreshFor(header, now, limit); got != 0 {
		t.Fatalf("HTTP freshness = %s; want 0", got)
	}
	if got := ViewFreshFor(header, now, limit); got != limit {
		t.Fatalf("view freshness = %s; want %s", got, limit)
	}
	header.Add("Cache-Control", "max-age=10")
	if got := ViewFreshFor(header, now, limit); got != 10*time.Second {
		t.Fatalf("view freshness = %s; want 10s", got)
	}
	header.Add("Cache-Control", "no-cache")
	if got := ViewFreshFor(header, now, limit); got != 0 {
		t.Fatalf("view freshness for no-cache = %s; want 0", got)
	}
}
