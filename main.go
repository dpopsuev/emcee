package main

import (
	"fmt"
	"os"

	"github.com/DanyPops/emcee/internal/adapter/driver/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
