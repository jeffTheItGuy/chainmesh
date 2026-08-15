package model

import "time"

type Usage struct {
	TenantID string    `json:"tenant_id"`
	Method   string    `json:"method"`
	Count    int64     `json:"count"`
	BytesIn  int64     `json:"bytes_in"`
	Period   time.Time `json:"period"`
}
