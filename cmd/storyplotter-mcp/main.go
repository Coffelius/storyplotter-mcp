package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gabistuff/storyplotter-mcp/internal/data"
	"github.com/gabistuff/storyplotter-mcp/internal/mcp"
)

const Version = "0.1.0"

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(0)
	log.SetPrefix("[storyplotter-mcp] ")

	fmt.Fprintf(os.Stderr, "storyplotter-mcp v%s\n", Version)

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
	if err := srv.ServeStdio(os.Stdin, os.Stdout); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
