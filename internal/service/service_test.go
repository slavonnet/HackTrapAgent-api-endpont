package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"hacktrapagent-api-endpoint/internal/filter"
	"hacktrapagent-api-endpoint/internal/model"
	"hacktrapagent-api-endpoint/internal/ratelimit"
)

type fakeStore struct {
	inserted []model.EventRecord
	err      error
}

func (f *fakeStore) EnsureSchema(context.Context) error { return nil }
func (f *fakeStore) LoadRecent(context.Context, time.Time) ([]model.EventRecord, error) {
	return nil, nil
}
func (f *fakeStore) Close() error { return nil }
func (f *fakeStore) InsertEvent(_ context.Context, event model.EventRecord) error {
	if f.err != nil {
		return f.err
	}
	f.inserted = append(f.inserted, event)
	return nil
}

func TestEventServiceDefaultActionAndStore(t *testing.T) {
	store := &fakeStore{}
	limiter, _ := ratelimit.New(nil)
	now := time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)
	svc := New(Dependencies{
		Store:   store,
		Limiter: limiter,
		Clock: func() time.Time {
			return now
		},
	})

	code, err := svc.HandleEvent(context.Background(), model.EventPayload{
		MashineID: "m-1",
		SrcIP:     "10.0.0.1",
		DstIP:     "8.8.8.8",
	}, "1.1.1.1")
	if err != nil {
		t.Fatalf("HandleEvent() error = %v", err)
	}
	if code != model.CodeOK {
		t.Fatalf("HandleEvent() code = %s, want %s", code, model.CodeOK)
	}
	if len(store.inserted) != 1 {
		t.Fatalf("insert count = %d, want 1", len(store.inserted))
	}
	if store.inserted[0].Action != "deny" {
		t.Fatalf("action = %s, want deny", store.inserted[0].Action)
	}
}

func TestEventServiceBlacklist(t *testing.T) {
	store := &fakeStore{}
	limiter, _ := ratelimit.New(nil)
	svc := New(Dependencies{
		Store:   store,
		Limiter: limiter,
		Blacklist: []filter.Rule{
			{"source": "5.5.5.5"},
		},
	})

	code, err := svc.HandleEvent(context.Background(), model.EventPayload{
		MashineID: "m-1",
		SrcIP:     "10.0.0.2",
		DstIP:     "8.8.4.4",
	}, "5.5.5.5")
	if err != nil {
		t.Fatalf("HandleEvent() error = %v", err)
	}
	if code != model.CodeAccessDenied {
		t.Fatalf("code = %s, want %s", code, model.CodeAccessDenied)
	}
	if len(store.inserted) != 0 {
		t.Fatalf("insert count = %d, want 0", len(store.inserted))
	}
}

func TestEventServiceWhitelistBypassBlacklistAndLimiter(t *testing.T) {
	store := &fakeStore{}
	limiter, _ := ratelimit.New([]ratelimit.Rule{
		{Keys: []string{"source"}, WindowSecond: 60, Limit: 1},
	})
	svc := New(Dependencies{
		Store:   store,
		Limiter: limiter,
		Blacklist: []filter.Rule{
			{"source": "6.6.6.6"},
		},
		Whitelist: []filter.Rule{
			{"source": "6.6.6.6"},
		},
		Clock: time.Now,
	})

	for i := 0; i < 2; i++ {
		code, err := svc.HandleEvent(context.Background(), model.EventPayload{
			MashineID: "m-1",
			SrcIP:     "10.0.0.3",
			DstIP:     "9.9.9.9",
		}, "6.6.6.6")
		if err != nil {
			t.Fatalf("HandleEvent() error = %v", err)
		}
		if code != model.CodeOK {
			t.Fatalf("code = %s, want %s", code, model.CodeOK)
		}
	}
	if len(store.inserted) != 2 {
		t.Fatalf("insert count = %d, want 2", len(store.inserted))
	}
}

func TestEventServiceRateLimit(t *testing.T) {
	store := &fakeStore{}
	limiter, _ := ratelimit.New([]ratelimit.Rule{
		{Keys: []string{"source"}, WindowSecond: 60, Limit: 1},
	})
	now := time.Now()
	svc := New(Dependencies{
		Store:   store,
		Limiter: limiter,
		Clock: func() time.Time {
			return now
		},
	})

	first, err := svc.HandleEvent(context.Background(), model.EventPayload{
		MashineID: "m-1",
		SrcIP:     "10.0.0.5",
		DstIP:     "1.1.1.1",
	}, "7.7.7.7")
	if err != nil || first != model.CodeOK {
		t.Fatalf("first = (%s, %v), want (ok,nil)", first, err)
	}

	second, err := svc.HandleEvent(context.Background(), model.EventPayload{
		MashineID: "m-1",
		SrcIP:     "10.0.0.5",
		DstIP:     "1.1.1.1",
	}, "7.7.7.7")
	if err != nil {
		t.Fatalf("second error = %v", err)
	}
	if second != model.CodeRateLimit {
		t.Fatalf("second code = %s, want %s", second, model.CodeRateLimit)
	}
}

func TestEventServiceStoreError(t *testing.T) {
	store := &fakeStore{err: errors.New("db down")}
	limiter, _ := ratelimit.New(nil)
	svc := New(Dependencies{
		Store:   store,
		Limiter: limiter,
	})

	code, err := svc.HandleEvent(context.Background(), model.EventPayload{
		MashineID: "m-1",
		SrcIP:     "10.0.0.8",
		DstIP:     "8.8.8.8",
	}, "2.2.2.2")
	if err == nil {
		t.Fatalf("expected error")
	}
	if code != model.CodeError {
		t.Fatalf("code = %s, want %s", code, model.CodeError)
	}
}
