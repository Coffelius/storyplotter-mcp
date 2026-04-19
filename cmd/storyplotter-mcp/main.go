package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/Coffelius/storyplotter-mcp/internal/data"
	"github.com/Coffelius/storyplotter-mcp/internal/mcp"
	"github.com/Coffelius/storyplotter-mcp/internal/tools"
)

// envInt64 reads a positive int64 env var, falling back to def on unset or
// parse error. A warning is logged on parse error.
func envInt64(name string, def int64) int64 {
	raw := os.Getenv(name)
	if raw == "" {
		return def
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v < 0 {
		log.Printf("warning: %s=%q invalid, using default %d", name, raw, def)
		return def
	}
	return v
}

func envInt(name string, def int) int {
	return int(envInt64(name, int64(def)))
}

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
		cfg := mcp.HTTPConfig{
			Addr:                    *addr,
			Bearer:                  os.Getenv("MCP_BEARER"),
			BodyLimit:               envInt64("MCP_BODY_LIMIT_BYTES", 5242880),
			MCPRateLimitPerMin:      envInt("MCP_RATE_LIMIT_PER_MIN", 60),
			DownloadRateLimitPerMin: envInt("DOWNLOAD_RATE_LIMIT_PER_MIN", 30),
		}
		log.Printf("http listening on %s (body<=%d, mcp<=%d/min, download<=%d/min)",
			cfg.Addr, cfg.BodyLimit, cfg.MCPRateLimitPerMin, cfg.DownloadRateLimitPerMin)
		if err := srv.ServeHTTP(cfg); err != nil {
			log.Fatalf("serve http: %v", err)
		}
	default:
		log.Fatalf("unknown mode: %s", *mode)
	}
}
