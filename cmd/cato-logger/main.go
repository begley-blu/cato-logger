package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cato-logger/internal/api"
	"cato-logger/internal/cef"
	"cato-logger/internal/config"
	"cato-logger/internal/logging"
	"cato-logger/internal/marker"
	"cato-logger/internal/preflight"
	"cato-logger/internal/processor"
	"cato-logger/internal/syslog"
)

const version = "3.2"

func main() {
	// Create cancellable context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load configuration from JSON
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize structured logger
	logger, err := logging.New(cfg.LogLevel, cfg.LogFormat, cfg.LogOutput)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Close()

	// Startup banner
	logger.Info("starting Cato Networks CEF Forwarder",
		"version", version,
		"pid", os.Getpid(),
		"config_file", cfg.ConfigPath)

	logger.Info("configuration loaded",
		"api_url", cfg.CatoAPIURL,
		"account_id", cfg.CatoAccountID,
		"syslog_server", cfg.SyslogAddress(),
		"syslog_protocol", cfg.SyslogProtocol,
		"fetch_interval_sec", cfg.FetchInterval,
		"max_events", cfg.MaxEvents,
		"max_pagination", cfg.MaxPagination,
		"log_level", cfg.LogLevel,
		"log_format", cfg.LogFormat)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		logger.Error("configuration validation failed", "error", err.Error())
		os.Exit(1)
	}

	// Run pre-flight checks
	logger.Info("running pre-flight checks")
	preflightChecker := preflight.New(logger)
	preflightResults := preflightChecker.RunAll(
		cfg.CatoAPIURL,
		cfg.CatoAPIKey,
		cfg.CatoAccountID,
		cfg.SyslogProtocol,
		cfg.SyslogAddress(),
		cfg.MarkerFile,
		time.Duration(cfg.ConnTimeout)*time.Second,
	)

	if preflight.HasFailures(preflightResults) {
		logger.Error("pre-flight checks failed, cannot start service")
		fmt.Fprintf(os.Stderr, "\n%s\n", preflight.FormatFailures(preflightResults))
		os.Exit(1)
	}

	logger.Info("all pre-flight checks passed")

	// Initialize marker manager
	markerMgr, err := marker.New(cfg.MarkerFile, logger)
	if err != nil {
		logger.Error("failed to initialize marker manager", "error", err.Error())
		os.Exit(1)
	}

	// Initialize CEF formatter
	cefFormatter := cef.NewFormatter(
		cfg.CEFVendor,
		cfg.CEFProduct,
		cfg.CEFVersion,
		cfg.FieldMappings,
		cfg.OrderedFields,
	)
	logger.Info("CEF formatter initialized",
		"vendor", cfg.CEFVendor,
		"product", cfg.CEFProduct,
		"field_mappings", len(cfg.FieldMappings))

	// Initialize API client
	apiClient := api.NewClient(
		cfg.CatoAPIURL,
		cfg.CatoAPIKey,
		cfg.CatoAccountID,
		time.Duration(cfg.ConnTimeout)*time.Second,
		logger,
	)

	// Initialize syslog writer
	syslogWriter, err := syslog.NewWriter(
		cfg.SyslogProtocol,
		cfg.SyslogAddress(),
		time.Duration(cfg.ConnTimeout)*time.Second,
		logger,
	)
	if err != nil {
		logger.Error("failed to initialize syslog connection", "error", err.Error())
		os.Exit(1)
	}
	defer syslogWriter.Close()

	// Initialize stats tracker
	stats := processor.NewStats()

	// Initialize processor
	proc := processor.New(cfg, apiClient, syslogWriter, cefFormatter, markerMgr, stats, logger)

	logger.Info("all components initialized successfully")

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP)

	// Main service loop with exponential backoff
	ticker := time.NewTicker(time.Duration(cfg.FetchInterval) * time.Second)
	defer ticker.Stop()

	backoffDelay := 1 * time.Second
	maxBackoff := time.Duration(cfg.MaxBackoffDelay) * time.Second

	logger.Info("starting main processing loop")

	// Process initial events immediately
	success := proc.ProcessWithRecovery(ctx)
	if !success {
		logger.Warn("initial processing cycle failed, will retry")
	}

	for {
		select {
		case <-ctx.Done():
			logger.Info("context cancelled, shutting down")
			return

		case <-ticker.C:
			success := proc.ProcessWithRecovery(ctx)

			if success {
				// Reset backoff on success
				if backoffDelay > 1*time.Second {
					logger.Info("processing recovered, resetting backoff")
				}
				backoffDelay = 1 * time.Second
				ticker.Reset(time.Duration(cfg.FetchInterval) * time.Second)
			} else {
				// Apply exponential backoff on failure
				logger.Warn("processing failed, applying backoff",
					"backoff_delay", backoffDelay.String(),
					"next_attempt_in", backoffDelay.String())
				ticker.Reset(backoffDelay)
				backoffDelay *= 2
				if backoffDelay > maxBackoff {
					backoffDelay = maxBackoff
				}
			}

		case sig := <-sigChan:
			logger.Info("received signal", "signal", sig.String())

			if sig == syscall.SIGHUP {
				logger.Info("SIGHUP received - configuration reload not yet implemented")
				// Note: With JSON config, we could reload the entire config here
				// For now, just log it
				continue
			}

			// Save final state and shutdown
			logger.Info("initiating graceful shutdown")

			// Log final statistics
			logger.Info("final statistics",
				"total_events_forwarded", stats.GetTotalEvents(),
				"total_api_requests", stats.GetTotalAPIRequests(),
				"failed_api_requests", stats.GetFailedAPIRequests())

			cancel()
			return
		}
	}
}
