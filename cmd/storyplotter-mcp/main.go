package main

import (
	"fmt"
	"os"
)

const Version = "0.1.0"

func main() {
	fmt.Fprintf(os.Stderr, "storyplotter-mcp v%s\n", Version)
	os.Exit(0)
}
