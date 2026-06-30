// Copyright 2024 The Infrix Authors
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

// Command infrix-playground serves the hosted playground: a no-install browser
// experience that asks an Infrix node to run a golden governed flow, streams it,
// exports a portable proof, and verifies that proof IN THE BROWSER (no node
// trust). Anonymous Demo mode is on by default; Kermit Sandbox mode is opt-in
// and only meaningful when the configured node offers live Kermit runs. It never
// writes to mainnet and stores no private keys.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/opendlt/infrix-playground/api"
)

// envOr returns env var name's value, or def when unset/empty. It lets a
// container configure the playground via env (12-factor); CLI flags override.
func envOr(name, def string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return def
}

// envBool parses a boolean env var (1/true/yes); unset/unparseable → false.
func envBool(name string) bool {
	b, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(name)))
	return err == nil && b
}

func main() {
	addr := flag.String("addr", envOr("INFRIX_PLAYGROUND_ADDR", "127.0.0.1:8086"), "address to listen on")
	nodeEndpoint := flag.String("node-endpoint", envOr("INFRIX_PLAYGROUND_NODE_ENDPOINT", "http://127.0.0.1:8080"), "base URL of the Infrix node that runs flows (POST /v4/playground/runFlow)")
	receiptDir := flag.String("receipt-dir", envOr("INFRIX_PLAYGROUND_RECEIPT_DIR", ""), "directory to persist shared receipts (empty = in-memory only)")
	kermit := flag.Bool("kermit", envBool("INFRIX_PLAYGROUND_KERMIT"), "advertise live Kermit Sandbox mode (the configured node must offer it; testnet only)")
	kermitL0 := flag.String("kermit-l0", "kermit", "Kermit network name used for the mainnet-refusal check; mainnet is refused")
	rateBurst := flag.Float64("rate-burst", 0, "rate limit burst per client (0 = default)")
	ratePerMin := flag.Float64("rate-per-min", 0, "rate limit sustained requests per minute per client (0 = default)")
	flag.Parse()

	cfg := api.DefaultConfig()
	cfg.Addr = *addr
	cfg.RunFlowEndpoint = *nodeEndpoint
	cfg.ReceiptDir = *receiptDir
	if *rateBurst > 0 || *ratePerMin > 0 {
		def := api.DefaultRateLimitConfig()
		if *rateBurst > 0 {
			def.Burst = *rateBurst
		}
		if *ratePerMin > 0 {
			def.PerMinute = *ratePerMin
		}
		cfg.RateLimit = def
	}

	if *kermit {
		// Testnet-only: refuse anything that names mainnet.
		if strings.Contains(strings.ToLower(*kermitL0), "mainnet") {
			fmt.Fprintln(os.Stderr, "playground: mainnet is refused; use the Kermit testnet")
			os.Exit(1)
		}
		// The playground is a thin client: live Kermit runs are executed by the
		// configured node (which owns the L0 anchor wiring). The operator
		// asserts the node offers them.
		cfg.KermitAvailable = true
	}

	srv, err := api.NewServer(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "playground:", err)
		os.Exit(1)
	}

	mode := "Anonymous Demo only (fixture-backed)"
	if cfg.KermitEnabled() {
		mode = "Anonymous Demo + Kermit Sandbox (live testnet)"
	}
	fmt.Printf("Infrix playground on http://%s/  [%s]\n", cfg.Addr, mode)
	fmt.Printf("Runs execute on the node at %s; proofs are verified in your browser (no node trust).\n", cfg.RunFlowEndpoint)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := srv.Run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "playground:", err)
		os.Exit(1)
	}
}
