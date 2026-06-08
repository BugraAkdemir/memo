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
)

func main() {
	port := flag.Int("port", 8090, "Port for headless REST API server")
	flag.Parse()

	app := NewApp()

	// Create a context that will be cancelled on SIGINT/SIGTERM
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app.startup(ctx)
	defer app.shutdown(ctx)

	// Start the REST API server for Flutter frontend (plain HTTP, no TLS)
	app.startWebServerHTTP(*port)
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
