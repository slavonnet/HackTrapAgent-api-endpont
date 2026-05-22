package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"hacktrapagent-api-endpoint/internal/model"
)

type EventProcessor interface {
	HandleEvent(ctx context.Context, payload model.EventPayload, source string) (string, error)
}

type Handler struct {
	processor        EventProcessor
	requestBodyLimit int64
}

func NewHandler(processor EventProcessor, requestBodyLimit int64) *Handler {
	return &Handler{
		processor:        processor,
		requestBodyLimit: requestBodyLimit,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/event" {
		writeResponse(w, http.StatusNotFound, model.CodeParseError)
		return
	}
	if r.Method != http.MethodPost {
		writeResponse(w, http.StatusMethodNotAllowed, model.CodeParseError)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.requestBodyLimit)
	defer r.Body.Close()

	var payload model.EventPayload
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeResponse(w, http.StatusBadRequest, model.CodeParseError)
		return
	}

	code, err := h.processor.HandleEvent(r.Context(), payload, requestSourceIP(r))
	if err != nil {
		slog.Error("event processing failed", "error", err)
		writeResponse(w, http.StatusInternalServerError, model.CodeError)
		return
	}

	switch code {
	case model.CodeOK:
		writeResponse(w, http.StatusOK, code)
	case model.CodeRateLimit:
		writeResponse(w, http.StatusTooManyRequests, code)
	case model.CodeAccessDenied:
		writeResponse(w, http.StatusForbidden, code)
	case model.CodeMashineIDNotFound, model.CodeDstIPNotFound, model.CodeParseError:
		writeResponse(w, http.StatusBadRequest, code)
	default:
		writeResponse(w, http.StatusInternalServerError, model.CodeError)
	}
}

func requestSourceIP(r *http.Request) string {
	forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0])
	if ip := net.ParseIP(forwarded); ip != nil {
		return ip.String()
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		if ip := net.ParseIP(host); ip != nil {
			return ip.String()
		}
	}

	return strings.TrimSpace(r.RemoteAddr)
}

func writeResponse(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"code": code,
	})
}
