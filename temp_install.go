package main

import (
	"context"
	"fmt"
	"log"
	"memo/internal/llama"
)

func main() {
	i := llama.NewInstaller("data")
	
	logger := func(line string) {
		fmt.Println("[INSTALL]", line)
	}

	fmt.Println("Starting CPU-only llama.cpp installation...")
	path, err := i.Install(context.Background(), logger)
	if err != nil {
		log.Fatalf("Installation failed: %v", err)
	}

	fmt.Printf("\nSuccess! llama-server installed at: %s\n", path)
}
