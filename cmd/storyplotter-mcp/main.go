package main

import (
	"encoding/hex"
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

	var signer *mcp.TokenSigner
	publicURL := os.Getenv("MCP_PUBLIC_URL")

	switch *mode {
	case "stdio":
		srv := mcp.NewServer(store, nil, publicURL)
		for _, t := range tools.All() {
			srv.Register(t)
		}
		if err := srv.ServeStdio(os.Stdin, os.Stdout); err != nil {
			log.Fatalf("serve stdio: %v", err)
		}
	case "http":
		keyHex := os.Getenv("DOWNLOAD_SIGNING_KEY")
		if keyHex == "" {
			log.Fatalf("DOWNLOAD_SIGNING_KEY is required in http mode (hex-encoded, >=32 bytes, generate with: openssl rand -hex 32)")
		}
		key, err := hex.DecodeString(keyHex)
		if err != nil {
			log.Fatalf("DOWNLOAD_SIGNING_KEY: invalid hex: %v", err)
		}
		if len(key) < 16 {
			log.Fatalf("DOWNLOAD_SIGNING_KEY: decoded key is %d bytes; need >=16", len(key))
		}
		signer = mcp.NewTokenSigner(key)

		srv := mcp.NewServer(store, signer, publicURL)
		for _, t := range tools.All() {
			srv.Register(t)
		}
		cfg := mcp.HTTPConfig{Addr: *addr, Bearer: os.Getenv("MCP_BEARER")}
		log.Printf("http listening on %s (publicURL=%q)", cfg.Addr, publicURL)
		if err := srv.ServeHTTP(cfg); err != nil {
			log.Fatalf("serve http: %v", err)
		}
	default:
		log.Fatalf("unknown mode: %s", *mode)
	}
}
