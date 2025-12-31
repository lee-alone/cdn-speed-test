package main

import (
	"cloudflare-speedtest/internal/server"
	"cloudflare-speedtest/internal/yamlconfig"
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

//go:embed static
var staticFS embed.FS

func main() {
	// Get the directory of the running binary
	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get executable path: %v", err)
	}
	exeDir := filepath.Dir(exePath)

	// Load or create configuration
	configPath := filepath.Join(exeDir, "config.yaml")
	cfg, err := yamlconfig.LoadAndValidate(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Configuration loaded from: %s\n", configPath)

	// Create and start server
	srv := server.New(cfg, exeDir, configPath, staticFS)

	fmt.Printf("Starting Cloudflare Speed Test server...\n")
	fmt.Printf("Data directory: %s\n", exeDir)
	fmt.Println("Open http://localhost:8080 in your browser")

	if err := srv.Run(":8080"); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
