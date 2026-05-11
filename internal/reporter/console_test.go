package reporter

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadFile(t *testing.T) {
	type testCaseT struct {
		err         error
		setup       func(t *testing.T) string
		description string
		output      string
	}

	testCases := []testCaseT{
		{
			// File exists and has readable content.
			description: "successful read returns file content",
			setup: func(t *testing.T) string {
				t.Helper()
				path := filepath.Join(t.TempDir(), "valid.txt")
				require.NoError(t, os.WriteFile(path, []byte("hello world"), 0o644))
				return path
			},
			output: "hello world",
		},
		{
			// Path does not exist so os.Open fails.
			description: "non-existent file returns error from os.Open",
			setup: func(t *testing.T) string {
				t.Helper()
				return filepath.Join(t.TempDir(), "does-not-exist.txt")
			},
			err: syscall.ENOENT,
		},
		{
			// Path is a directory so os.Open succeeds but io.ReadAll fails.
			description: "directory path returns error from io.ReadAll",
			setup: func(t *testing.T) string {
				t.Helper()
				return t.TempDir()
			},
			err: syscall.EISDIR,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			path := tc.setup(t)
			content, err := readFile(path)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.output, content)
		})
	}
}
