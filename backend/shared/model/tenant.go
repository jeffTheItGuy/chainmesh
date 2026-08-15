package model

import "time"

type Tenant struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	APIKey    string    `json:"-"`
	QuotaRPM  int       `json:"quota_rpm"`
	CreatedAt time.Time `json:"created_at"`
}
