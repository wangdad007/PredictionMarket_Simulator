package main

import (
	"os"

	"predictionmarket-simulator/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, true))
}
