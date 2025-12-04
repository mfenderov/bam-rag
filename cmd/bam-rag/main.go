package main

import (
	"fmt"
	"os"

	"github.com/mfenderov/bam-rag/cmd/bam-rag/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
