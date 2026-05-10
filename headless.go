package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func runHeadless(port int) {
	app := NewApp()

	// Create a context that will be cancelled on SIGINT/SIGTERM
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app.startup(ctx)
	defer app.shutdown(ctx)

	// Start the web server for Flutter frontend (API only, no static assets needed)
	app.startWebServer(port)
	log.Printf("Memo headless server running on port %d", port)

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	fmt.Println("\nShutting down...")
}

func init() {
	headless := flag.Bool("headless", false, "Run without GUI (REST API server only)")
	port := flag.Int("port", 8090, "Port for headless REST API server")
	flag.Parse()

	if *headless {
		runHeadless(*port)
		os.Exit(0)
	}
}
