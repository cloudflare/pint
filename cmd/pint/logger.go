package main

import (
	"fmt"

	"github.com/cloudflare/pint/internal/log"
)

func initLogger(level string, noColor bool) error {
	l, err := log.ParseLevel(level)
	if err != nil {
		return fmt.Errorf("'%s' is not a valid log level", level)
	}

	log.Setup(l, noColor)

	return nil
}
