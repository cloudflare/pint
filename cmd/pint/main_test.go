package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// mock command that fails tests if error is returned
func mockMainShouldSucceed() int {
	app := newApp()
	err := app.Run(os.Args)
	if err != nil {
		log.WithLevel(zerolog.FatalLevel).Err(err).Msg("Fatal error")
		return 1
	}
	return 0
}

// mock command that fails tests if no error is returned
func mockMainShouldFail() int {
	app := newApp()
	err := app.Run(os.Args)
	if err != nil {
		log.WithLevel(zerolog.FatalLevel).Err(err).Msg("Fatal error")
		return 0
	}
	fmt.Fprintf(os.Stderr, "expected an error but none was returned\n")
	return 1
}

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"pint.ok":    mockMainShouldSucceed,
		"pint.error": mockMainShouldFail,
	}))
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:           "tests",
		UpdateScripts: os.Getenv("UPDATE_SNAPSHOTS") == "1",
		Setup: func(env *testscript.Env) error {
			// inject an env variable with the current working directory
			// so we can use it to copy files into testscript workdir
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			env.Vars = append(env.Vars, fmt.Sprintf("TESTCWD=%s", cwd))
			return nil
		},
	})
}
