package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/harry-urek/urek/v/internal/config"
	loggerPkg "github.com/harry-urek/urek/v/internal/logger"
	"github.com/harry-urek/urek/v/internal/session"
	grpcTransport "github.com/harry-urek/urek/v/internal/transport/grpc"
	httpTransport "github.com/harry-urek/urek/v/internal/transport/http"
	"github.com/harry-urek/urek/v/pkg/monitoring"
	"go.uber.org/zap"
)

var (
	cfg     *config.Config
	appLog  *zap.Logger
	manager *session.Manager
)

func init() {
	var err error

	cfg, err = config.LoadConfig("./config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	appLog = loggerPkg.InitLogger(&cfg.Logger)

	// Create session manager
	manager, err = session.NewManager(cfg)
	if err != nil {
		appLog.Fatal("Failed to create session manager", zap.Error(err))
	}

	appLog.Info("Initialization completed successfully")
}

func main() {
	defer appLog.Sync()

	appLog.Info("Starting Kratos Session Management Service")

	httpServer := httpTransport.NewServer(manager)
	httpHandler := monitoring.Middleware(httpServer.Handler())

	httpAddr := fmt.Sprintf(":%d", cfg.Server.HTTPPort)
	srv := &http.Server{
		Addr:         httpAddr,
		Handler:      httpHandler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		appLog.Info("Starting HTTP server", zap.String("address", httpAddr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLog.Fatal("HTTP server failed", zap.Error(err))
		}
	}()

	// gRPC server
	grpcServer := grpcTransport.NewServer(manager)
	grpcAddr := fmt.Sprintf(":%d", cfg.Server.GRPCPort)
	go func() {
		appLog.Info("Starting gRPC server", zap.String("address", grpcAddr))
		if err := grpcServer.Start(grpcAddr); err != nil {
			appLog.Fatal("gRPC server failed", zap.Error(err))
		}
	}()

	if cfg.Monitoring.Enabled {
		metricsAddr := fmt.Sprintf(":%d", cfg.Monitoring.Port)
		metricsServer := &http.Server{
			Addr:    metricsAddr,
			Handler: monitoring.Handler(),
		}
		go func() {
			appLog.Info("Starting metrics server", zap.String("address", metricsAddr))
			if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				appLog.Error("Metrics server failed", zap.Error(err))
			}
		}()
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLog.Info("Shutting down servers...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		appLog.Error("HTTP server shutdown failed", zap.Error(err))
	}

	grpcServer.Stop()

	appLog.Info("Servers stopped")
}
