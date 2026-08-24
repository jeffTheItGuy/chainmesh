		testClient := blockchain.New(endpoints)
		resp, err := testClient.Call(r.Context(), "eth_chainId")
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"connected": false,
				"error":     "connection failed",
			})
			return
		}

		var rpcResp blockchain.RPCResponse
		if err := json.Unmarshal(resp, &rpcResp); err != nil || rpcResp.Error != nil {
			msg := "invalid rpc response"
			if rpcResp.Error != nil {
				msg = rpcResp.Error.Message
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"connected": false,
				"error":     msg,
			})
			return
		}

		chainIDHex := strings.Trim(string(rpcResp.Result), `"`)
		chainIDDec, _ := strconv.ParseInt(chainIDHex, 0, 64)

		writeJSON(w, http.StatusOK, map[string]any{
			"connected": true,
			"chain_id":  strconv.FormatInt(chainIDDec, 10),
		})
	}))

	mux.HandleFunc("/blockchain", adminAuth(secret, db, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			configs, err := db.ListBlockchainConfigs(r.Context())
			if err != nil {
				writeErr(w, http.StatusInternalServerError, "database error")
				return
			}
			writeJSON(w, http.StatusOK, configs)
		case http.MethodPost:
			r.Body = http.MaxBytesReader(w, r.Body, maxAdminBodyBytes)
			var req struct {
				Name         string `json:"name"`
				RPCEndpoint1 string `json:"rpc_endpoint_1"`
				RPCEndpoint2 string `json:"rpc_endpoint_2"`
				ChainID      string `json:"chain_id"`
				Enabled      bool   `json:"enabled"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeErr(w, http.StatusBadRequest, "invalid json")
				return
			}
			if req.Name == "" || req.RPCEndpoint1 == "" {
				writeErr(w, http.StatusBadRequest, "name and rpc_endpoint_1 are required")
				return
			}
			if err := util.ValidateRPCEndpoint(req.RPCEndpoint1); err != nil {
				writeErr(w, http.StatusBadRequest, "invalid rpc_endpoint_1: "+err.Error())
				return
			}
			if req.RPCEndpoint2 != "" {
				if err := util.ValidateRPCEndpoint(req.RPCEndpoint2); err != nil {
					writeErr(w, http.StatusBadRequest, "invalid rpc_endpoint_2: "+err.Error())
					return
				}
			}

			cfg := &model.BlockchainConfig{
				Name:         req.Name,
				RPCEndpoint1: req.RPCEndpoint1,
				RPCEndpoint2: req.RPCEndpoint2,
				ChainID:      req.ChainID,
				Enabled:      req.Enabled,
			}
			saved, err := db.SaveBlockchainConfig(r.Context(), cfg)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, "database error")
				return
			}

			auditLog(db, r, "CREATE_BLOCKCHAIN_CONFIG", "blockchain_config", saved.ID, map[string]any{
				"name":     saved.Name,
				"chain_id": saved.ChainID,
			})

			writeJSON(w, http.StatusCreated, saved)
		default:
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	}))

	mux.HandleFunc("/blockchain/", adminAuth(secret, db, func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/blockchain/")
		if id == "" {
			writeErr(w, http.StatusBadRequest, "network id required")
			return
		}
		switch r.Method {
		case http.MethodGet:
			cfg, err := db.GetBlockchainConfig(r.Context(), id)
			if err != nil {
				writeErr(w, http.StatusNotFound, "not found")
				return
			}
			writeJSON(w, http.StatusOK, cfg)
		case http.MethodPut:
			r.Body = http.MaxBytesReader(w, r.Body, maxAdminBodyBytes)
			var req struct {
				Name         string `json:"name"`
				RPCEndpoint1 string `json:"rpc_endpoint_1"`
				RPCEndpoint2 string `json:"rpc_endpoint_2"`
				ChainID      string `json:"chain_id"`
				Enabled      bool   `json:"enabled"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeErr(w, http.StatusBadRequest, "invalid json")
				return
			}
			if req.Name == "" || req.RPCEndpoint1 == "" {
				writeErr(w, http.StatusBadRequest, "name and rpc_endpoint_1 are required")
				return
			}
			if err := util.ValidateRPCEndpoint(req.RPCEndpoint1); err != nil {
				writeErr(w, http.StatusBadRequest, "invalid rpc_endpoint_1: "+err.Error())
				return
			}
			if req.RPCEndpoint2 != "" {
				if err := util.ValidateRPCEndpoint(req.RPCEndpoint2); err != nil {
					writeErr(w, http.StatusBadRequest, "invalid rpc_endpoint_2: "+err.Error())
					return
				}
			}

			cfg := &model.BlockchainConfig{
				ID:           id,
				Name:         req.Name,
				RPCEndpoint1: req.RPCEndpoint1,
				RPCEndpoint2: req.RPCEndpoint2,
				ChainID:      req.ChainID,
				Enabled:      req.Enabled,
			}
			if err := db.UpdateBlockchainConfig(r.Context(), cfg); err != nil {
				writeErr(w, http.StatusInternalServerError, "database error")
				return
			}

			auditLog(db, r, "UPDATE_BLOCKCHAIN_CONFIG", "blockchain_config", id, map[string]any{
				"name":     req.Name,
				"chain_id": req.ChainID,
			})

			writeJSON(w, http.StatusOK, map[string]any{"updated": true})
		case http.MethodDelete:
			if err := db.DeleteBlockchainConfig(r.Context(), id); err != nil {
				writeErr(w, http.StatusInternalServerError, "database error")
				return
			}

			auditLog(db, r, "DELETE_BLOCKCHAIN_CONFIG", "blockchain_config", id, nil)

			writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
		default:
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	}))

	mux.HandleFunc("/gateway/health/nodes", func(w http.ResponseWriter, r *http.Request) {
		gatewayAddr := os.Getenv("GATEWAY_ADDR")
		if gatewayAddr == "" {
			gatewayAddr = "http://localhost:8080"
		}

		resp, err := http.Get(gatewayAddr + "/health/nodes")
		if err != nil {
			writeErr(w, http.StatusBadGateway, "gateway unreachable")
			return
		}
		defer resp.Body.Close()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	})

	mux.HandleFunc("/audit-logs", adminAuth(secret, db, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		limit := 50
		offset := 0
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 {
				limit = n
			}
		}
		if o := r.URL.Query().Get("offset"); o != "" {
			if n, err := strconv.Atoi(o); err == nil && n >= 0 {
				offset = n
			}
		}

		logs, err := db.ListAuditLogs(r.Context(), limit, offset)
		if err != nil {
			log.Error("audit logs query failed", "err", err)
			writeErr(w, http.StatusInternalServerError, "database error")
			return
		}
		writeJSON(w, http.StatusOK, logs)
	}))

	return mux
}

func main() {
	log := logger.New()
	adminSecret := os.Getenv("ADMIN_SECRET")
	if adminSecret == "" {
		log.Error("ADMIN_SECRET is not set - refusing to start with an unauthenticated admin API")
		os.Exit(1)
	}

	db, err := postgres.New(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Error("postgres failed", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	mux := newAdminMux(adminSecret, db, log)

	srv := &http.Server{
		Addr:              ":8081",
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	tlsCert := os.Getenv("TLS_CERT")
	tlsKey := os.Getenv("TLS_KEY")

	go func() {
		log.Info("admin starting", "addr", srv.Addr, "tls", tlsCert != "" && tlsKey != "")
		var err error
		if tlsCert != "" && tlsKey != "" {
			err = srv.ListenAndServeTLS(tlsCert, tlsKey)
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			log.Error("server failed", "err", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}