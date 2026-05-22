package ratelimit

import (
	"testing"
	"time"
)

func TestLimiterAllow(t *testing.T) {
	limiter, err := New([]Rule{
		{
			Keys:         []string{"source", "mashine_id"},
			WindowSecond: 10,
			Limit:        2,
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	now := time.Now()
	values := map[string]string{
		"source":     "10.0.0.1",
		"mashine_id": "machine-1",
	}
	if !limiter.Allow(values, now) {
		t.Fatalf("first request must pass")
	}
	if !limiter.Allow(values, now.Add(time.Second)) {
		t.Fatalf("second request must pass")
	}
	if limiter.Allow(values, now.Add(2*time.Second)) {
		t.Fatalf("third request must be blocked")
	}
}

func TestLimiterSeed(t *testing.T) {
	limiter, err := New([]Rule{
		{
			Keys:         []string{"source"},
			WindowSecond: 5,
			Limit:        1,
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	now := time.Now()
	values := map[string]string{"source": "10.0.0.2"}
	limiter.Seed(values, now.Add(-2*time.Second), now)
	if limiter.Allow(values, now) {
		t.Fatalf("request should be blocked after seeding")
	}
}
