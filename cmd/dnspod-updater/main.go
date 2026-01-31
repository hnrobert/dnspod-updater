package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/hnrobert/dnspod-updater/internal/config"
	"github.com/hnrobert/dnspod-updater/internal/dnspod"
	"github.com/hnrobert/dnspod-updater/internal/ipdetect"
	"github.com/hnrobert/dnspod-updater/internal/updater"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	cfg, err := config.FromEnv()
	if err != nil {
		log.Printf("config error: %v", err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ipDetector := ipdetect.NewDetector(ipdetect.Options{
		PreferredIface: cfg.IPPreferredIface,
		Method:         cfg.IPDetectMethod,
	})

	client := dnspod.NewClient(dnspod.ClientOptions{
		HTTPTimeout: cfg.HTTPTimeout,
		BaseURL:     cfg.DNSPodBaseURL,
		UserAgent:   cfg.UserAgent,
	})

	u := updater.New(updater.Options{
		Config:     cfg,
		Detector:   ipDetector,
		DNSPod:     client,
		Logger:     log.Default(),
		StartDelay: cfg.StartDelay,
	})

	if err := u.Run(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		log.Printf("fatal: %v", err)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
