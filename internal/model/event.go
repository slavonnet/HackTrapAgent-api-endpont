package model

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

const (
	CodeOK                = "ok"
	CodeAccessDenied      = "access denided"
	CodeParseError        = "parse error"
	CodeError             = "error"
	CodeRateLimit         = "ratelimit"
	CodeMashineIDNotFound = "mashine_id not found"
	CodeDstIPNotFound     = "dst_ip not found"
)

type EventPayload struct {
	EventDatetime string          `json:"event_datetime"`
	MashineID     string          `json:"mashine_id"`
	ContainerID   string          `json:"container_id"`
	UnitName      string          `json:"unit_name"`
	Hostname      string          `json:"hostname"`
	ID            string          `json:"id"`
	DstIP         string          `json:"dst_ip"`
	DstFQDN       string          `json:"dst_fqdn"`
	SrcIP         string          `json:"src_ip"`
	SrcPort       *uint16         `json:"src_port"`
	DstPort       *uint16         `json:"dst_port"`
	Protocol      string          `json:"protocol"`
	ServicePort   *uint16         `json:"service_port"`
	Action        string          `json:"action"`
	Extra         json.RawMessage `json:"extra"`
}

type EventRecord struct {
	EventDatetime time.Time
	RegisteredAt  time.Time
	Source        string
	MashineID     string
	ContainerID   *string
	UnitName      *string
	Hostname      *string
	ID            *string
	DstIP         *string
	DstFQDN       *string
	SrcIP         string
	SrcPort       *uint16
	DstPort       *uint16
	Protocol      *string
	ServicePort   *uint16
	Action        string
	Extra         *string
}

func PtrIfNotEmpty(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(value)
	return &trimmed
}

func (r EventRecord) ToValuesMap() map[string]string {
	values := map[string]string{
		"source":     r.Source,
		"mashine_id": r.MashineID,
		"src_ip":     r.SrcIP,
		"action":     r.Action,
	}

	putPtrValue(values, "container_id", r.ContainerID)
	putPtrValue(values, "unit_name", r.UnitName)
	putPtrValue(values, "hostname", r.Hostname)
	putPtrValue(values, "id", r.ID)
	putPtrValue(values, "dst_ip", r.DstIP)
	putPtrValue(values, "dst_fqdn", r.DstFQDN)
	putPtrValue(values, "protocol", r.Protocol)

	if r.SrcPort != nil {
		values["src_port"] = uint16ToString(*r.SrcPort)
	}
	if r.DstPort != nil {
		values["dst_port"] = uint16ToString(*r.DstPort)
	}
	if r.ServicePort != nil {
		values["service_port"] = uint16ToString(*r.ServicePort)
	}

	return values
}

func putPtrValue(values map[string]string, key string, value *string) {
	if value != nil {
		values[key] = *value
	}
}

func uint16ToString(v uint16) string {
	return strconv.FormatUint(uint64(v), 10)
}
