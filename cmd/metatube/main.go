package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/metatube-community/metatube-sdk-go/imageutil"
	"github.com/metatube-community/metatube-sdk-go/internal/service"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
func run() error {
	if err := imageutil.NativeWebP(); err != nil {
		if os.Getenv("REQUIRE_NATIVE_WEBP") == "1" {
			return fmt.Errorf("native WebP backend required: %w", err)
		}
		log.Print("WebP backend: pure Go")
	} else {
		log.Print("WebP backend: native libwebp")
	}

	cfg, err := service.LoadConfig()
	if err != nil {
		return err
	}
	handler, closeDB, err := service.Open(cfg)
	if err != nil {
		return err
	}
	defer closeDB()
	server := &http.Server{Addr: cfg.Address, Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, WriteTimeout: cfg.RequestTimeout + 5*time.Second, MaxHeaderBytes: 16 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	failures := make(chan error, 1)
	go func() { failures <- server.ListenAndServe() }()
	log.Printf("MetaTube listening on %s; API documentation at /docs", cfg.Address)
	select {
	case err := <-failures:
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
