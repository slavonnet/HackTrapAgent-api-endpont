package service

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"time"

	"hacktrapagent-api-endpoint/internal/filter"
	"hacktrapagent-api-endpoint/internal/model"
	"hacktrapagent-api-endpoint/internal/ratelimit"
)

type EventStore interface {
	EnsureSchema(ctx context.Context) error
	InsertEvent(ctx context.Context, event model.EventRecord) error
	LoadRecent(ctx context.Context, since time.Time) ([]model.EventRecord, error)
	Close() error
}

type Dependencies struct {
	Store     EventStore
	Limiter   *ratelimit.Limiter
	Blacklist []filter.Rule
	Whitelist []filter.Rule
	Clock     func() time.Time
}

type EventService struct {
	store     EventStore
	limiter   *ratelimit.Limiter
	blacklist []filter.Rule
	whitelist []filter.Rule
	clock     func() time.Time
}

func New(deps Dependencies) *EventService {
	clock := deps.Clock
	if clock == nil {
		clock = time.Now
	}

	return &EventService{
		store:     deps.Store,
		limiter:   deps.Limiter,
		blacklist: deps.Blacklist,
		whitelist: deps.Whitelist,
		clock:     clock,
	}
}

func (s *EventService) HandleEvent(ctx context.Context, payload model.EventPayload, source string) (string, error) {
	event, values, code := s.parsePayload(payload, source)
	if code != model.CodeOK {
		return code, nil
	}

	if filter.MatchesAny(values, s.whitelist) {
		return s.storeEvent(ctx, event)
	}

	if filter.MatchesAny(values, s.blacklist) {
		return model.CodeAccessDenied, nil
	}

	if s.limiter != nil && !s.limiter.Allow(values, s.clock()) {
		return model.CodeRateLimit, nil
	}

	return s.storeEvent(ctx, event)
}

func (s *EventService) storeEvent(ctx context.Context, event model.EventRecord) (string, error) {
	if err := s.store.InsertEvent(ctx, event); err != nil {
		return model.CodeError, err
	}
	return model.CodeOK, nil
}

func (s *EventService) parsePayload(payload model.EventPayload, source string) (model.EventRecord, map[string]string, string) {
	now := s.clock().UTC()

	if strings.TrimSpace(payload.MashineID) == "" {
		return model.EventRecord{}, nil, model.CodeMashineIDNotFound
	}

	if strings.TrimSpace(payload.SrcIP) == "" {
		return model.EventRecord{}, nil, model.CodeParseError
	}
	if net.ParseIP(strings.TrimSpace(payload.SrcIP)) == nil {
		return model.EventRecord{}, nil, model.CodeParseError
	}

	if strings.TrimSpace(payload.DstIP) == "" && strings.TrimSpace(payload.DstFQDN) == "" {
		return model.EventRecord{}, nil, model.CodeDstIPNotFound
	}
	if strings.TrimSpace(payload.DstIP) != "" && net.ParseIP(strings.TrimSpace(payload.DstIP)) == nil {
		return model.EventRecord{}, nil, model.CodeParseError
	}

	eventDatetime, err := parseEventDatetime(payload.EventDatetime, now)
	if err != nil {
		return model.EventRecord{}, nil, model.CodeParseError
	}

	action := strings.TrimSpace(strings.ToLower(payload.Action))
	if action == "" {
		action = "deny"
	}

	record := model.EventRecord{
		EventDatetime: eventDatetime,
		RegisteredAt:  now,
		Source:        source,
		MashineID:     strings.TrimSpace(payload.MashineID),
		ContainerID:   model.PtrIfNotEmpty(payload.ContainerID),
		UnitName:      model.PtrIfNotEmpty(payload.UnitName),
		Hostname:      model.PtrIfNotEmpty(payload.Hostname),
		ID:            model.PtrIfNotEmpty(payload.ID),
		DstIP:         model.PtrIfNotEmpty(payload.DstIP),
		DstFQDN:       model.PtrIfNotEmpty(payload.DstFQDN),
		SrcIP:         strings.TrimSpace(payload.SrcIP),
		SrcPort:       payload.SrcPort,
		DstPort:       payload.DstPort,
		Protocol:      model.PtrIfNotEmpty(strings.ToLower(payload.Protocol)),
		ServicePort:   payload.ServicePort,
		Action:        action,
		Extra:         normalizeExtra(payload.Extra),
	}

	return record, record.ToValuesMap(), model.CodeOK
}

func parseEventDatetime(raw string, fallback time.Time) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback, nil
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		ts, err := time.Parse(layout, value)
		if err == nil {
			return ts.UTC(), nil
		}
	}
	return time.Time{}, errors.New("invalid event_datetime")
}

func normalizeExtra(raw json.RawMessage) *string {
	if len(raw) == 0 {
		return nil
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	value := trimmed
	return &value
}
