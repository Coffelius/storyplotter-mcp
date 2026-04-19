package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/gabistuff/storyplotter-mcp/internal/data"
	"github.com/gabistuff/storyplotter-mcp/internal/mcp"
	"github.com/gabistuff/storyplotter-mcp/internal/tools"
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

	path := os.Getenv("STORYPLOTTER_DATA_PATH")
	var exp *data.Export
	if path != "" {
		e, err := data.Load(path)
		if err != nil {
			log.Fatalf("load data: %v", err)
		}
		exp = e
		log.Printf("loaded %d plots from %s", len(exp.PlotList), path)
	} else {
		log.Printf("STORYPLOTTER_DATA_PATH not set; running with empty data")
		exp = &data.Export{}
	}

	srv := mcp.NewServer(exp)
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
