package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/Coffelius/storyplotter-mcp/internal/data"
	"github.com/Coffelius/storyplotter-mcp/internal/mcp"
	"github.com/Coffelius/storyplotter-mcp/internal/tools"
)

const Version = "0.1.0"

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(0)
	log.SetPrefix("[storyplotter-mcp] ")

	mode := flag.String("mode", "stdio", "transport mode: stdio or http")
	addr := flag.String("addr", ":8080", "listen address for http mode")
	flag.Parse()

	fmt.Fprintf(os.Stderr, "storyplotter-mcp v%s (mode=%s)\n", Version, *mode)

	sharedPath := os.Getenv("STORYPLOTTER_DATA_PATH")
	dataDir := os.Getenv("STORYPLOTTER_DATA_DIR")
	if dataDir == "" {
		dataDir = "/data/users"
	}
	log.Printf("user store: baseDir=%s shared=%s", dataDir, sharedPath)

	store := data.NewDiskUserStore(dataDir, sharedPath)
	srv := mcp.NewServer(store)
	for _, t := range tools.All() {
		srv.Register(t)
	}

	switch *mode {
	case "stdio":
		if err := srv.ServeStdio(os.Stdin, os.Stdout); err != nil {
			log.Fatalf("serve stdio: %v", err)
		}
	case "http":
		cfg := mcp.HTTPConfig{Addr: *addr, Bearer: os.Getenv("MCP_BEARER")}
		log.Printf("http listening on %s", cfg.Addr)
		if err := srv.ServeHTTP(cfg); err != nil {
			log.Fatalf("serve http: %v", err)
		}
	default:
		log.Fatalf("unknown mode: %s", *mode)
	}
}
