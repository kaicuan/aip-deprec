package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"aip/internal/config"
	"aip/internal/db"
	"aip/internal/server"
)

func main() {
	cfg, err := config.GetConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	command := os.Args[1]

	switch command {
	case "start":
		if err := startDaemon(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error starting AIP daemon: %v\n", err)
			os.Exit(1)
		}

	case "stop":
		if err := stopDaemon(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error stopping AIP daemon: %v\n", err)
			os.Exit(1)
		}

	case "status":
		if err := daemonStatus(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error getting status: %v\n", err)
			os.Exit(1)
		}

	case "__run_daemon":
		runForeground(cfg)

	case "help", "--help", "-h":
		printUsage()

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %q\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func runForeground(cfg *config.Config) {
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("Fatal: failed to open SQLite database: %v", err)
	}
	defer database.Close()

	srv := server.NewServer(cfg, database)

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down AIP Gateway...")
		_ = srv.Shutdown()
		_ = os.Remove(cfg.PIDPath)
		os.Exit(0)
	}()

	if err := srv.Start(); err != nil && err.Error() != "http: Server closed" {
		log.Fatalf("Fatal: server crashed: %v", err)
	}
}

func printUsage() {
	fmt.Println("AIP - Dedicated OpenCode Free Proxy Gateway & Web UI")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  aip start    Start the background gateway daemon")
	fmt.Println("  aip stop     Gracefully terminate the background daemon")
	fmt.Println("  aip status   Display current running state and proxy metrics")
	fmt.Println()
	fmt.Println("Web Dashboard:")
	fmt.Println("  http://localhost:9100")
}
