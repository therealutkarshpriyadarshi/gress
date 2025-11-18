package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/therealutkarshpriyadarshi/gress/pkg/config"
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

	// TODO: Initialize stream processing engine with cfg.ToEngineConfig()
	// TODO: Set up metrics server with cfg.Metrics
	// TODO: Initialize sources from cfg.Sources
	// TODO: Initialize sinks from cfg.Sinks
	// TODO: Start processing

	logger.Info("Gress is running. Press Ctrl+C to stop.",
		zap.Int("buffer_size", cfg.Engine.BufferSize),
		zap.Int("max_concurrency", cfg.Engine.MaxConcurrency),
		zap.String("state_backend", cfg.State.Backend),
		zap.Bool("metrics_enabled", cfg.Metrics.Enabled))

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down gracefully...")
	// TODO: Cleanup resources
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
	var zapLevel zap.AtomicLevel

	switch level {
	case "debug":
		zapLevel = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		zapLevel = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		zapLevel = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		zapLevel = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		zapLevel = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	logger.Info("Log level updated", zap.String("new_level", level))
}
