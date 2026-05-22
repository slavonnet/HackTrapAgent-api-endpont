package ratelimit

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type Rule struct {
	Keys         []string `json:"keys"`
	WindowSecond int      `json:"window"`
	Limit        int      `json:"limit"`
}

type Limiter struct {
	mu      sync.Mutex
	rules   []Rule
	buckets map[string][]time.Time
}

func New(rules []Rule) (*Limiter, error) {
	normalized := make([]Rule, 0, len(rules))
	for idx, rule := range rules {
		if len(rule.Keys) == 0 {
			return nil, fmt.Errorf("limits[%d].keys must not be empty", idx)
		}
		if rule.WindowSecond <= 0 {
			return nil, fmt.Errorf("limits[%d].window must be positive", idx)
		}
		if rule.Limit <= 0 {
			return nil, fmt.Errorf("limits[%d].limit must be positive", idx)
		}

		keys := append([]string{}, rule.Keys...)
		for i := range keys {
			keys[i] = strings.TrimSpace(keys[i])
			if keys[i] == "" {
				return nil, fmt.Errorf("limits[%d].keys contains empty item", idx)
			}
		}
		sort.Strings(keys)
		normalized = append(normalized, Rule{
			Keys:         keys,
			WindowSecond: rule.WindowSecond,
			Limit:        rule.Limit,
		})
	}

	return &Limiter{
		rules:   normalized,
		buckets: make(map[string][]time.Time),
	}, nil
}

func (l *Limiter) MaxWindow() time.Duration {
	max := 0
	for _, rule := range l.rules {
		if rule.WindowSecond > max {
			max = rule.WindowSecond
		}
	}
	return time.Duration(max) * time.Second
}

func (l *Limiter) Allow(values map[string]string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	for idx, rule := range l.rules {
		bucketKey := composeBucketKey(idx, rule, values)
		cutoff := now.Add(-time.Duration(rule.WindowSecond) * time.Second)
		kept := pruneOld(l.buckets[bucketKey], cutoff)
		if len(kept) >= rule.Limit {
			l.buckets[bucketKey] = kept
			return false
		}
		kept = append(kept, now)
		l.buckets[bucketKey] = kept
	}

	return true
}

func (l *Limiter) Seed(values map[string]string, eventTime, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for idx, rule := range l.rules {
		cutoff := now.Add(-time.Duration(rule.WindowSecond) * time.Second)
		if eventTime.Before(cutoff) {
			continue
		}
		bucketKey := composeBucketKey(idx, rule, values)
		kept := pruneOld(l.buckets[bucketKey], cutoff)
		kept = append(kept, eventTime)
		l.buckets[bucketKey] = kept
	}
}

func composeBucketKey(idx int, rule Rule, values map[string]string) string {
	parts := make([]string, 0, len(rule.Keys)+2)
	parts = append(parts, fmt.Sprintf("rule:%d", idx))
	parts = append(parts, fmt.Sprintf("window:%d", rule.WindowSecond))
	for _, key := range rule.Keys {
		parts = append(parts, key+"="+values[key])
	}
	return strings.Join(parts, "|")
}

func pruneOld(input []time.Time, cutoff time.Time) []time.Time {
	if len(input) == 0 {
		return input
	}

	out := input[:0]
	for _, ts := range input {
		if !ts.Before(cutoff) {
			out = append(out, ts)
		}
	}
	return out
}
