package main

import (
	"fmt"
	"os"

	"github.com/dpopsuev/emcee/internal/api/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
