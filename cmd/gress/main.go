package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/config"
	"github.com/therealutkarshpriyadarshi/gress/pkg/metrics"
	"github.com/therealutkarshpriyadarshi/gress/pkg/tracing"
	"go.uber.org/zap"
)

var (
	configFile = flag.String("config", "config.yaml", "Path to configuration file")
	validate   = flag.Bool("validate", false, "Validate configuration and exit")
)

const version = "1.0.0"

func main() {
	flag.Parse()

	// Load configuration with environment variable overrides
	cfg, err := config.LoadOrDefaultWithEnv(*configFile)
	if err != nil {
		panic(err)
	}

	// Validate configuration
	if err := config.Validate(cfg); err != nil {
		panic(err)
	}

	// If validate flag is set, exit after validation
	if *validate {
		println("Configuration is valid")
		return
	}

	// Initialize logger based on configuration
	logger := initLogger(cfg.Logging)
	defer logger.Sync()

	logger.Info("Starting Gress Stream Processing System",
		zap.String("version", version),
		zap.String("config_file", *configFile),
		zap.String("config_version", cfg.Version),
		zap.String("app_name", cfg.Application.Name),
		zap.String("environment", cfg.Application.Environment))

	// Create reloadable configuration for hot reload support
	reloadableConfig, err := config.NewReloadableConfig(*configFile, logger)
	if err != nil {
		logger.Warn("Failed to create reloadable config, hot reload disabled", zap.Error(err))
	} else {
		// Register callback for configuration reloads
		reloadableConfig.OnReload(func(oldCfg, newCfg *config.Config) error {
			logger.Info("Configuration reloaded",
				zap.String("old_log_level", oldCfg.Logging.Level),
				zap.String("new_log_level", newCfg.Logging.Level),
				zap.Duration("old_checkpoint_interval", oldCfg.Engine.CheckpointInterval),
				zap.Duration("new_checkpoint_interval", newCfg.Engine.CheckpointInterval))

			// Update logger level if changed
			if oldCfg.Logging.Level != newCfg.Logging.Level {
				updateLogLevel(logger, newCfg.Logging.Level)
			}

			return nil
		})

		// Start configuration watcher
		reloadableConfig.Start()
		defer reloadableConfig.Stop()

		logger.Info("Configuration hot reload enabled")
	}

	// Initialize metrics collector
	var metricsServer *metrics.Server
	var runtimeCollector *metrics.RuntimeCollector
	var systemCollector *metrics.SystemCollector

	if cfg.Metrics.Enabled {
		metricsCollector := metrics.NewCollector(logger)

		// Create runtime and system collectors
		runtimeCollector = metrics.NewRuntimeCollector(metricsCollector.GetRegistry(), logger)
		systemCollector = metrics.NewSystemCollector(metricsCollector.GetRegistry(), logger)

		// Start collectors
		runtimeCollector.Start(cfg.Engine.MetricsInterval)
		systemCollector.Start(cfg.Engine.MetricsInterval)

		// Start metrics HTTP server
		metricsServer = metrics.NewServer(cfg.Metrics.Address, metricsCollector, logger)
		if err := metricsServer.Start(); err != nil {
			logger.Fatal("Failed to start metrics server", zap.Error(err))
		}
		logger.Info("Metrics server started",
			zap.String("address", cfg.Metrics.Address),
			zap.String("path", cfg.Metrics.Path))
	}

	// Initialize distributed tracing
	var tracingProvider *tracing.TracerProvider
	if cfg.Tracing.Enabled {
		tracingConfig := &tracing.Config{
			Enabled:          cfg.Tracing.Enabled,
			ServiceName:      cfg.Tracing.ServiceName,
			ServiceVersion:   cfg.Tracing.ServiceVersion,
			Environment:      cfg.Tracing.Environment,
			SamplingRate:     cfg.Tracing.SamplingRate,
			ExporterType:     cfg.Tracing.ExporterType,
			ExporterEndpoint: cfg.Tracing.ExporterEndpoint,
			OTLPHeaders:      cfg.Tracing.OTLPHeaders,
			OTLPInsecure:     cfg.Tracing.OTLPInsecure,
		}

		tracingProvider, err = tracing.NewProvider(tracingConfig, logger)
		if err != nil {
			logger.Fatal("Failed to initialize tracing", zap.Error(err))
		}
	}

	// TODO: Initialize stream processing engine with cfg.ToEngineConfig()
	// TODO: Initialize sources from cfg.Sources
	// TODO: Initialize sinks from cfg.Sinks
	// TODO: Start processing

	logger.Info("Gress is running. Press Ctrl+C to stop.",
		zap.Int("buffer_size", cfg.Engine.BufferSize),
		zap.Int("max_concurrency", cfg.Engine.MaxConcurrency),
		zap.String("state_backend", cfg.State.Backend),
		zap.Bool("metrics_enabled", cfg.Metrics.Enabled),
		zap.Bool("tracing_enabled", cfg.Tracing.Enabled))

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down gracefully...")

	// Cleanup resources
	if runtimeCollector != nil {
		runtimeCollector.Stop()
	}
	if systemCollector != nil {
		systemCollector.Stop()
	}
	if metricsServer != nil {
		if err := metricsServer.Stop(); err != nil {
			logger.Error("Error stopping metrics server", zap.Error(err))
		}
	}
	if tracingProvider != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := tracingProvider.Shutdown(ctx); err != nil {
			logger.Error("Error shutting down tracing", zap.Error(err))
		}
	}

	logger.Info("Shutdown complete")
}

func initLogger(logConfig config.LoggingConfig) *zap.Logger {
	var logLevel zap.AtomicLevel

	switch logConfig.Level {
	case "debug":
		logLevel = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		logLevel = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		logLevel = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		logLevel = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		logLevel = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	var zapConfig zap.Config
	if logConfig.Format == "console" {
		zapConfig = zap.NewDevelopmentConfig()
	} else {
		zapConfig = zap.NewProductionConfig()
	}

	zapConfig.Level = logLevel

	// Configure output
	switch logConfig.Output {
	case "stderr":
		zapConfig.OutputPaths = []string{"stderr"}
	case "file":
		if logConfig.OutputPath != "" {
			zapConfig.OutputPaths = []string{logConfig.OutputPath}
		}
	default: // stdout
		zapConfig.OutputPaths = []string{"stdout"}
	}

	logger, err := zapConfig.Build()
	if err != nil {
		panic(err)
	}

	return logger
}

func updateLogLevel(logger *zap.Logger, level string) {
	// Note: This logs the level change but does not actually update the logger
	// For dynamic log level updates, the logger would need to be created with
	// zap.NewAtomicLevel() and the atomic level would need to be stored
	logger.Info("Log level updated", zap.String("new_level", level))
}
