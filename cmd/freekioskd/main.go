package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/k1LoW/errors"

	"github.com/AnaTofuZ/freekiosk-manager/internal/config"
	"github.com/AnaTofuZ/freekiosk-manager/internal/web"
)

func run() error {
	path := flag.String("config", config.DefaultPath(), "Configuration JSON file")
	addr := os.Getenv("FREEKIOSK_LISTEN")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	listen := flag.String("listen", addr, "Listen address")
	timeout := flag.Duration("timeout", 8*time.Second, "Device request timeout")
	flag.Parse()
	cfg, err := config.Load(*path)
	if err != nil {
		return err
	}
	handler, err := web.New(cfg, *timeout)
	if err != nil {
		return err
	}
	server := &http.Server{Addr: *listen, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: *timeout + 10*time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	log.Printf("FreeKiosk manager listening on %s", *listen)
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
}
func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
