package main

import (
	"fmt"
	"os"

	"github.com/cloudflare/pint/internal/log"
)

func initLogger(level string, noColor bool) error {
	l, err := log.ParseLevel(level)
	if err != nil {
		return fmt.Errorf("'%s' is not a valid log level", level)
	}

	nc := os.Getenv("NO_COLOR")
	if nc != "" && nc != "0" {
		noColor = true
	}

	log.Setup(l, noColor)

	return nil
}
