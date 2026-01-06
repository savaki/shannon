// // Copyright 2026 savaki
// //
// // Licensed under the Apache License, Version 2.0 (the "License");
// // you may not use this file except in compliance with the License.
// // You may obtain a copy of the License at
// //
// //     http://www.apache.org/licenses/LICENSE-2.0
// //
// // Unless required by applicable law or agreed to in writing, software
// // distributed under the License is distributed on an "AS IS" BASIS,
// // WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// // See the License for the specific language governing permissions and
// // limitations under the License.

// Package main provides the shannon CLI entry point.
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/savaki/shannon/internal/api"
	"github.com/savaki/shannon/internal/auth"
	"github.com/savaki/shannon/internal/config"
	"github.com/savaki/shannon/internal/git"
	"github.com/savaki/shannon/internal/logger"
	"github.com/savaki/shannon/internal/session"
	"github.com/savaki/shannon/internal/storage"
	"github.com/savaki/shannon/internal/tailscale"
	"github.com/urfave/cli/v2"
)

var version = "dev"

func main() {
	app := &cli.App{
		Name:    "shannon",
		Usage:   "Remote Claude Code control server",
		Version: version,
		Commands: []*cli.Command{
			serveCommand(),
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func serveCommand() *cli.Command {
	cfg := config.DefaultConfig()

	return &cli.Command{
		Name:  "serve",
		Usage: "Start the shannon server",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "host",
				Aliases:     []string{"H"},
				Usage:       "Hostname for QR code (auto-detected from Tailscale if not set)",
				EnvVars:     []string{"SHANNON_HOST"},
				Destination: &cfg.Host,
			},
			&cli.IntFlag{
				Name:        "port",
				Aliases:     []string{"p"},
				Usage:       "HTTP server port",
				Value:       cfg.Port,
				EnvVars:     []string{"SHANNON_PORT"},
				Destination: &cfg.Port,
			},
			&cli.StringFlag{
				Name:        "data-dir",
				Aliases:     []string{"d"},
				Usage:       "Data directory for SQLite and sessions",
				Value:       cfg.DataDir,
				EnvVars:     []string{"SHANNON_DATA_DIR"},
				Destination: &cfg.DataDir,
			},
			&cli.StringFlag{
				Name:        "repo",
				Aliases:     []string{"r"},
				Usage:       "Git repository URL to clone for sessions",
				EnvVars:     []string{"SHANNON_REPO"},
				Destination: &cfg.Repo,
			},
			&cli.BoolFlag{
				Name:        "worktree",
				Usage:       "Use git worktrees instead of full clones",
				Value:       cfg.UseWorktree,
				EnvVars:     []string{"SHANNON_WORKTREE"},
				Destination: &cfg.UseWorktree,
			},
			&cli.StringFlag{
				Name:        "firebase-creds",
				Usage:       "Path to Firebase service account JSON",
				EnvVars:     []string{"SHANNON_FIREBASE_CREDS"},
				Destination: &cfg.FirebaseCreds,
			},
			&cli.StringFlag{
				Name:        "log-level",
				Usage:       "Log verbosity (debug, info, warn, error)",
				Value:       cfg.LogLevel,
				EnvVars:     []string{"SHANNON_LOG_LEVEL"},
				Destination: &cfg.LogLevel,
			},
			&cli.BoolFlag{
				Name:        "tls",
				Usage:       "Enable HTTPS using Tailscale certificates",
				EnvVars:     []string{"SHANNON_TLS"},
				Destination: &cfg.TLS,
			},
		},
		Action: func(c *cli.Context) error {
			return runServe(cfg)
		},
	}
}

func runServe(cfg *config.Config) error {
	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	// Initialize structured logging
	logger.Init(cfg.LogLevel)

	// Ensure data directory exists
	if err := cfg.EnsureDataDir(); err != nil {
		return err
	}

	slog.Info("starting shannon server",
		"port", cfg.Port,
		"data_dir", cfg.DataDir,
		"log_level", cfg.LogLevel,
	)
	if cfg.Repo != "" {
		slog.Info("repository configured",
			"repo", cfg.Repo,
			"use_worktrees", cfg.UseWorktree,
		)
	}

	// Initialize storage
	dbPath := filepath.Join(cfg.DataDir, "shannon.db")
	store, err := storage.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Close()

	// Initialize git manager
	gitMgr := git.NewManager(cfg.DataDir, cfg.UseWorktree)

	// Initialize session manager
	sessionMgr := session.NewManager(store, gitMgr, cfg.DataDir)

	// Initialize auth
	pskMgr := auth.NewPSKManager(store)
	psk, err := pskMgr.GetOrCreate()
	if err != nil {
		return fmt.Errorf("failed to initialize auth: %w", err)
	}

	// Display QR code for mobile connection
	// Use configured host, or try Tailscale, or fall back to localhost
	// For TLS, use full FQDN (required for cert validation)
	// For HTTP, use short hostname (works with MagicDNS)
	host := cfg.Host
	if host == "" {
		if cfg.TLS {
			host = tailscale.GetHostnameOrDefault("localhost")
		} else {
			host = tailscale.GetShortHostnameOrDefault("localhost")
		}
	}
	qrConfig := auth.ConnectionConfig{
		Host: host,
		Port: cfg.Port,
		PSK:  psk,
		TLS:  cfg.TLS,
	}
	if err := auth.PrintQR(qrConfig); err != nil {
		slog.Warn("failed to print QR code", "error", err)
	}

	// Initialize API server
	addr := fmt.Sprintf(":%d", cfg.Port)
	server := api.NewServer(addr, sessionMgr, pskMgr)

	// Configure TLS if enabled
	if cfg.TLS {
		slog.Info("fetching Tailscale TLS certificate", "host", host)
		cert, err := tailscale.GetCertificate(host)
		if err != nil {
			return fmt.Errorf("failed to get TLS certificate: %w", err)
		}
		server.SetTLSCertificate(cert)
		slog.Info("TLS certificate loaded (in-memory only)")
	}

	// Start server in background
	serverErr := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil {
			serverErr <- err
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		return fmt.Errorf("server error: %w", err)
	case sig := <-sigCh:
		slog.Info("received shutdown signal", "signal", sig)
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown error: %w", err)
	}

	return nil
}
