package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hacktrapagent-api-endpoint/internal/model"
)

type fakeProcessor struct {
	code    string
	err     error
	payload model.EventPayload
	source  string
}

func (f *fakeProcessor) HandleEvent(_ context.Context, payload model.EventPayload, source string) (string, error) {
	f.payload = payload
	f.source = source
	return f.code, f.err
}

func TestHandlerSuccess(t *testing.T) {
	processor := &fakeProcessor{code: model.CodeOK}
	handler := NewHandler(processor, 1024)

	req := httptest.NewRequest(http.MethodPost, "/event", strings.NewReader(`{"mashine_id":"m1","src_ip":"10.0.0.1","dst_ip":"8.8.8.8"}`))
	req.RemoteAddr = "3.3.3.3:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if processor.source != "3.3.3.3" {
		t.Fatalf("source = %s, want 3.3.3.3", processor.source)
	}
	if processor.payload.MashineID != "m1" {
		t.Fatalf("payload mashine_id = %s, want m1", processor.payload.MashineID)
	}
}

func TestHandlerParseError(t *testing.T) {
	processor := &fakeProcessor{code: model.CodeOK}
	handler := NewHandler(processor, 1024)

	req := httptest.NewRequest(http.MethodPost, "/event", strings.NewReader(`{"mashine_id":`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandlerProcessorError(t *testing.T) {
	processor := &fakeProcessor{code: model.CodeError, err: errors.New("boom")}
	handler := NewHandler(processor, 1024)

	req := httptest.NewRequest(http.MethodPost, "/event", strings.NewReader(`{"mashine_id":"m1","src_ip":"10.0.0.1","dst_ip":"8.8.8.8"}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestHandlerRateLimit(t *testing.T) {
	processor := &fakeProcessor{code: model.CodeRateLimit}
	handler := NewHandler(processor, 1024)

	req := httptest.NewRequest(http.MethodPost, "/event", strings.NewReader(`{"mashine_id":"m1","src_ip":"10.0.0.1","dst_ip":"8.8.8.8"}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}
}
