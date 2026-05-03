package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aravindhsrbk/ims/internal/api"
	"github.com/aravindhsrbk/ims/internal/config"
	"github.com/aravindhsrbk/ims/internal/ingestion"
	"github.com/aravindhsrbk/ims/internal/metrics"
	"github.com/aravindhsrbk/ims/internal/storage/mongodb"
	"github.com/aravindhsrbk/ims/internal/storage/postgres"
	redisstore "github.com/aravindhsrbk/ims/internal/storage/redis"
	"github.com/aravindhsrbk/ims/internal/workflow"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags)
	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// --- Storage ---
	logger.Println("connecting to PostgreSQL...")
	pgClient, err := postgres.NewClient(cfg.PostgresDSN)
	if err != nil {
		logger.Fatalf("postgres: %v", err)
	}
	defer pgClient.Close()

	logger.Println("connecting to MongoDB...")
	mgClient, err := mongodb.NewClient(cfg.MongoURI, cfg.MongoDatabase)
	if err != nil {
		logger.Fatalf("mongodb: %v", err)
	}
	defer mgClient.Close()

	logger.Println("connecting to Redis...")
	rdClient, err := redisstore.NewClient(cfg.RedisAddr, cfg.RedisPassword)
	if err != nil {
		logger.Fatalf("redis: %v", err)
	}
	defer rdClient.Close()

	// --- Core components ---
	collector := metrics.NewCollector(logger)
	hub := api.NewHub(logger)
	buffer := ingestion.NewSignalBuffer(cfg.BufferSize)
	debouncer := ingestion.NewDebouncer(10 * time.Second)
	alerter := workflow.NewAlerterRegistry(logger)

	processor := ingestion.NewProcessor(
		buffer, debouncer,
		pgClient, mgClient, rdClient,
		alerter, hub,
		collector, cfg.WorkerCount,
	)

	// --- Start background goroutines ---
	go collector.Run(ctx)
	go hub.Run(ctx)
	go processor.Run(ctx)

	// --- HTTP server ---
	router := api.NewRouter(
		pgClient, mgClient, rdClient,
		buffer, processor, alerter,
		hub, collector,
		cfg.RateLimit, cfg.RateBurst,
		logger,
	)

	srv := &http.Server{
		Addr:         cfg.ServerAddr,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		logger.Printf("IMS backend listening on %s", cfg.ServerAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("server: %v", err)
		}
	}()

	// Graceful shutdown on SIGINT / SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Println("shutting down...")

	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Printf("server shutdown error: %v", err)
	}
	logger.Println("shutdown complete")
}
