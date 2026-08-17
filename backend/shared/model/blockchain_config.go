package model

import "time"

type BlockchainConfig struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	RPCEndpoint1 string    `json:"rpc_endpoint_1"`
	RPCEndpoint2 string    `json:"rpc_endpoint_2,omitempty"`
	ChainID      string    `json:"chain_id"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
