package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jeffTheItGuy/chainmesh/shared/blockchain"
	"github.com/jeffTheItGuy/chainmesh/shared/model"
	"github.com/jeffTheItGuy/chainmesh/shared/storage/postgres"
)

type NetworkHealth struct {
	NetworkID string                      `json:"network_id"`
	Nodes     []blockchain.EndpointHealth `json:"nodes"`
}

type Manager struct {
	db  *postgres.DB
	log *slog.Logger

	healthCtx    context.Context
	healthCancel context.CancelFunc

	mu      sync.RWMutex
	clients map[string]*blockchain.Client
	sigs    map[string]string
}

func NewManager(db *postgres.DB, log *slog.Logger) *Manager {
	healthCtx, healthCancel := context.WithCancel(context.Background())
	return &Manager{
		db:           db,
		log:          log,
		healthCtx:    healthCtx,
		healthCancel: healthCancel,
		clients:      make(map[string]*blockchain.Client),
		sigs:         make(map[string]string),
	}
}

func (m *Manager) Start(ctx context.Context) error {
	if err := m.reload(ctx); err != nil {
		return err
	}

	// FIX: Allow the gateway to start even if no blockchain networks are configured yet.
	// This prevents a chicken-and-egg problem where you can't access the Admin UI
	// to add a network because the gateway refuses to start without one.
	// The proxy layer already handles missing networks gracefully by returning
	// a 503 Service Unavailable error to clients if a request comes in.
	go m.loop()
	return nil
}

func (m *Manager) loop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.healthCtx.Done():
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(m.healthCtx, 10*time.Second)
			if err := m.reload(ctx); err != nil {
				m.log.Error("network reload failed", "err", err)
			}
			cancel()
		}
	}
}

func (m *Manager) reload(ctx context.Context) error {
	configs, err := m.db.ListBlockchainConfigs(ctx)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	desired := make(map[string]bool)
	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}
		desired[cfg.ID] = true

		sig := configSignature(cfg)
		oldSig, exists := m.sigs[cfg.ID]
		if exists && oldSig == sig {
			continue
		}

		if old, ok := m.clients[cfg.ID]; ok {
			old.StopHealthChecks()
		}

		endpoints := []string{cfg.RPCEndpoint1}
		if cfg.RPCEndpoint2 != "" {
			endpoints = append(endpoints, cfg.RPCEndpoint2)
		}

		client := blockchain.New(endpoints)
		client.SetNetworkID(cfg.ID)
		client.StartHealthChecks(m.healthCtx, 10*time.Second)

		m.clients[cfg.ID] = client
		m.sigs[cfg.ID] = sig

		m.log.Info(
			"blockchain client ready",
			"network", cfg.Name,
			"id", cfg.ID,
		)
	}

	for id, client := range m.clients {
		if !desired[id] {
			client.StopHealthChecks()
			delete(m.clients, id)
			delete(m.sigs, id)
			m.log.Info("blockchain client removed", "network_id", id)
		}
	}

	return nil
}

func (m *Manager) Get(networkID string) (*blockchain.Client, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, ok := m.clients[networkID]
	return client, ok
}

func (m *Manager) Health() []NetworkHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]NetworkHealth, 0, len(m.clients))
	for id, client := range m.clients {
		out = append(out, NetworkHealth{
			NetworkID: id,
			Nodes:     client.HealthyEndpoints(),
		})
	}
	return out
}

func (m *Manager) Stop() {
	m.healthCancel()

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, client := range m.clients {
		client.StopHealthChecks()
	}
	m.clients = make(map[string]*blockchain.Client)
	m.sigs = make(map[string]string)
}

func configSignature(cfg model.BlockchainConfig) string {
	return fmt.Sprintf("%s|%s|%t", cfg.RPCEndpoint1, cfg.RPCEndpoint2, cfg.Enabled)
}
