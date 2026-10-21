package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/ultimatewx/ultimatewx/internal/app"
	"github.com/ultimatewx/ultimatewx/internal/updater"
)

var version = "dev"
var repository = ""

func main() {
	defaultDir := os.Getenv("ULTIMATEWX_DATA_DIR")
	if defaultDir == "" {
		defaultDir = "./data"
	}
	configPath := flag.String("config", filepath.Join(defaultDir, "config.json"), "path to configuration JSON")
	dataDir := flag.String("data-dir", defaultDir, "directory for history and downloaded data")
	listen := flag.String("listen", "0.0.0.0:8080", "HTTP bind address")
	flag.Parse()
	updater.Version = version
	updater.Repository = repository
	application, e := app.New(*configPath, *dataDir)
	if e != nil {
		log.Fatalf("initialize UltimateWX: %v", e)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go application.Run(ctx)
	server := &http.Server{Addr: *listen, Handler: application.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 5 * time.Minute, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	go func() {
		<-ctx.Done()
		shutdown, c := context.WithTimeout(context.Background(), 10*time.Second)
		defer c()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("UltimateWX %s listening on http://%s", version, *listen)
	if e = server.ListenAndServe(); e != nil && e != http.ErrServerClosed {
		log.Fatalf("HTTP server: %v", e)
	}
}
