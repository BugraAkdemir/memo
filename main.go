package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"memo/internal/app"
)

func main() {
	port := flag.Int("port", 8090, "Port for headless REST API server")
	flag.Parse()

	a := app.NewApp(embeddedBinaries, versionFile)

	// Create a context that will be cancelled on SIGINT/SIGTERM
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	a.Startup(ctx)
	defer a.Shutdown(ctx)

	// Start the REST API server for Flutter frontend (plain HTTP, no TLS)
	if err := a.StartWebServerHTTP(*port); err != nil {
		log.Printf("FATAL: %v", err)
		os.Exit(1)
	}
	log.Printf("Memo backend server running on port %d", *port)

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	sigs := []os.Signal{os.Interrupt}
	if runtime.GOOS != "windows" {
		sigs = append(sigs, syscall.SIGTERM)
	}
	signal.Notify(sigCh, sigs...)
	<-sigCh
	fmt.Println("\nShutting down backend...")
}
