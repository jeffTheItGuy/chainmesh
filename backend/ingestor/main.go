package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jeffTheItGuy/chainmesh/shared/blockchain"
	"github.com/jeffTheItGuy/chainmesh/shared/logger"
	"github.com/jeffTheItGuy/chainmesh/shared/model"
	"github.com/jeffTheItGuy/chainmesh/shared/storage/postgres"
)

func main() {
	log := logger.New()
	db, err := postgres.New(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Error("postgres failed", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	configs, err := db.ListBlockchainConfigs(ctx)
	cancel()
	if err != nil {
		log.Error("failed to load blockchain configs", "err", err)
		os.Exit(1)
	}

	ingestCtx, ingestCancel := context.WithCancel(context.Background())
	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}
		go runIngestor(ingestCtx, cfg, db, log)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Info("shutting down ingestor")
	ingestCancel()
	time.Sleep(500 * time.Millisecond)
}

func runIngestor(ctx context.Context, cfg model.BlockchainConfig, db *postgres.DB, log *slog.Logger) {
	endpoints := []string{cfg.RPCEndpoint1}
	if cfg.RPCEndpoint2 != "" {
		endpoints = append(endpoints, cfg.RPCEndpoint2)
	}

	bc := blockchain.New(endpoints)
	bc.SetNetworkID(cfg.ID)
	bc.StartHealthChecks(ctx, 10*time.Second)
	defer bc.StopHealthChecks()

	ticker := time.NewTicker(12 * time.Second)
	defer ticker.Stop()

	log.Info(
		"ingestor started",
		"network", cfg.Name,
		"id", cfg.ID,
	)

	for {
		select {
		case <-ctx.Done():
			log.Info("ingestor stopped", "network", cfg.Name)
			return
		case <-ticker.C:
			if err := fetchAndStore(ctx, cfg.ID, bc, db, log); err != nil {
				log.Error(
					"fetch failed",
					"network", cfg.Name,
					"err", err,
				)
			}
		}
	}
}

func fetchAndStore(
	ctx context.Context,
	networkID string,
	bc *blockchain.Client,
	db *postgres.DB,
	log *slog.Logger,
) error {
	resp, err := bc.Call(ctx, "eth_getBlockByNumber", "latest", false)
	if err != nil {
		return err
	}

	var rpcResp blockchain.RPCResponse
	if err := json.Unmarshal(resp, &rpcResp); err != nil {
		return fmt.Errorf("invalid rpc response: %w", err)
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("rpc error: %s", rpcResp.Error.Message)
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
		NetworkID:  networkID,
		RawJSON:    raw["result"],
	}

	if err := db.StoreBlock(ctx, b); err != nil {
		log.Error(
			"store block failed",
			"network", networkID,
			"err", err,
		)
		return err
	}

	log.Info(
		"ingested block",
		"network", networkID,
		"number", num,
		"hash", block.Hash,
		"txs", len(block.Transactions),
	)

	return nil
}
