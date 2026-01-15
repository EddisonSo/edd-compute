package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"eddisonso.com/edd-compute/internal/api"
	"eddisonso.com/edd-compute/internal/db"
	"eddisonso.com/edd-compute/internal/k8s"
	"eddisonso.com/go-gfs/pkg/gfslog"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dbPath := flag.String("db", "/data/compute.db", "SQLite database path")
	logService := flag.String("log-service", "", "Log service address")
	flag.Parse()

	// Logger setup
	logger := gfslog.NewLogger(gfslog.Config{
		Source:         "edd-compute",
		LogServiceAddr: *logService,
		MinLevel:       slog.LevelDebug,
	})
	slog.SetDefault(logger.Logger)
	defer logger.Close()

	// Database
	database, err := db.Open(*dbPath)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	// K8s client (in-cluster config)
	k8sClient, err := k8s.NewClient()
	if err != nil {
		slog.Error("failed to create k8s client", "error", err)
		os.Exit(1)
	}

	// HTTP server
	handler := api.NewHandler(database, k8sClient)
	server := &http.Server{Addr: *addr, Handler: handler}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		slog.Info("shutting down")
		server.Close()
	}()

	slog.Info("edd-compute listening", "addr", *addr)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
