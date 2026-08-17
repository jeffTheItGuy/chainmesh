package model

import "time"

type Tenant struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	APIKey              string    `json:"-"`
	QuotaRPM            int       `json:"quota_rpm"`
	QuotaRPS            int       `json:"quota_rps"`
	QuotaDaily          int       `json:"quota_daily"`
	Plan                string    `json:"plan"`
	BlockchainNetworkID string    `json:"blockchain_network_id,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
}