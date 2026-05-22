package httpapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"hacktrapagent-api-endpoint/internal/model"
	"hacktrapagent-api-endpoint/internal/ratelimit"
	"hacktrapagent-api-endpoint/internal/service"
)

type perfStore struct {
	writes atomic.Int64
}

func (p *perfStore) EnsureSchema(context.Context) error { return nil }
func (p *perfStore) LoadRecent(context.Context, time.Time) ([]model.EventRecord, error) {
	return nil, nil
}
func (p *perfStore) Close() error { return nil }
func (p *perfStore) InsertEvent(context.Context, model.EventRecord) error {
	p.writes.Add(1)
	return nil
}

func TestEventEndpointHandlesAtLeast1kRPS(t *testing.T) {
	store := &perfStore{}
	limiter, err := ratelimit.New([]ratelimit.Rule{
		{
			Keys:         []string{"source", "mashine_id"},
			WindowSecond: 60,
			Limit:        1000000,
		},
	})
	if err != nil {
		t.Fatalf("ratelimit.New() error = %v", err)
	}

	svc := service.New(service.Dependencies{
		Store:   store,
		Limiter: limiter,
		Clock:   time.Now,
	})
	server := httptest.NewServer(NewHandler(svc, 1<<20))
	defer server.Close()

	const (
		totalRequests = 6000
		workers       = 200
		targetRPS     = 1000.0
	)

	tr := &http.Transport{
		MaxIdleConns:        workers * 2,
		MaxIdleConnsPerHost: workers * 2,
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   10 * time.Second,
	}
	defer tr.CloseIdleConnections()

	var statusErrors atomic.Int64
	var requestErrors atomic.Int64
	body := `{"mashine_id":"m-1","src_ip":"10.10.10.10","dst_ip":"8.8.8.8","protocol":"tcp","dst_port":53}`

	jobs := make(chan struct{}, totalRequests)
	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				req, err := http.NewRequest(http.MethodPost, server.URL+"/event", strings.NewReader(body))
				if err != nil {
					requestErrors.Add(1)
					continue
				}
				req.Header.Set("Content-Type", "application/json")

				resp, err := client.Do(req)
				if err != nil {
					requestErrors.Add(1)
					continue
				}
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					statusErrors.Add(1)
				}
			}
		}()
	}

	for i := 0; i < totalRequests; i++ {
		jobs <- struct{}{}
	}
	close(jobs)
	wg.Wait()

	elapsed := time.Since(start).Seconds()
	rps := float64(totalRequests) / elapsed
	t.Logf("load test: total=%d elapsed=%.3fs rps=%.2f status_errors=%d request_errors=%d writes=%d",
		totalRequests,
		elapsed,
		rps,
		statusErrors.Load(),
		requestErrors.Load(),
		store.writes.Load(),
	)

	if requestErrors.Load() != 0 {
		t.Fatalf("request errors: %d", requestErrors.Load())
	}
	if statusErrors.Load() != 0 {
		t.Fatalf("non-200 responses: %d", statusErrors.Load())
	}
	if store.writes.Load() != totalRequests {
		t.Fatalf("stored events: %d, want %d", store.writes.Load(), totalRequests)
	}
	if rps < targetRPS {
		t.Fatalf("RPS %.2f is below required %.2f", rps, targetRPS)
	}
}
