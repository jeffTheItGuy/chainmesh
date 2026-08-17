package model

import "time"

type Block struct {
	Number      int64     `json:"number"`
	Hash        string    `json:"hash"`
	ParentHash  string    `json:"parent_hash"`
	Timestamp   time.Time `json:"timestamp"`
	TxCount     int       `json:"tx_count"`
	NetworkID   string    `json:"network_id,omitempty"`
	NetworkName string    `json:"network_name,omitempty"`
	RawJSON     []byte    `json:"-"`
}
