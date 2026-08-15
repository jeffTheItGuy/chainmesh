package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/yourname/blockmesh/shared/blockchain"
	"github.com/yourname/blockmesh/shared/logger"
	"github.com/yourname/blockmesh/shared/model"
	"github.com/yourname/blockmesh/shared/storage/postgres"
)

func main() {
	log := logger.New()

	db, err := postgres.New(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Error("postgres failed", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	bc := blockchain.New([]string{os.Getenv("RPC_ENDPOINT_1")})

	ticker := time.NewTicker(12 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if err := fetchAndStore(context.Background(), bc, db, log); err != nil {
			log.Error("fetch failed", "err", err)
		}
	}
}

func fetchAndStore(ctx context.Context, bc *blockchain.Client, db *postgres.DB, log *slog.Logger) error {
	resp, err := bc.Call(ctx, "eth_getBlockByNumber", "latest", true)
	if err != nil {
		return err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(resp, &raw); err != nil {
		return err
	}

	var block struct {
		Number       string `json:"number"`
		Hash         string `json:"hash"`
		ParentHash   string `json:"parentHash"`
		Timestamp    string `json:"timestamp"`
		Transactions []any  `json:"transactions"`
	}
	if err := json.Unmarshal(raw["result"], &block); err != nil {
		return err
	}

	num, err := strconv.ParseInt(block.Number, 0, 64)
	if err != nil {
		return err
	}

	tsHex, err := strconv.ParseInt(block.Timestamp, 0, 64)
	if err != nil {
		return err
	}
	timestamp := time.Unix(tsHex, 0)

	b := &model.Block{
		Number:     num,
		Hash:       block.Hash,
		ParentHash: block.ParentHash,
		Timestamp:  timestamp,
		TxCount:    len(block.Transactions),
		RawJSON:    raw["result"],
	}

	if err := db.StoreBlock(ctx, b); err != nil {
		log.Error("store block failed", "err", err)
		return err
	}

	log.Info("ingested block", "number", num, "hash", block.Hash, "txs", len(block.Transactions))
	return nil
}
