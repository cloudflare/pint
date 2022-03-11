package parser_test

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/parser"
)

func TestReadContent(t *testing.T) {
	type testCaseT struct {
		input       []byte
		output      []byte
		shouldError bool
	}

	testCases := []testCaseT{
		{
			input:  []byte(""),
			output: []byte(""),
		},
		{
			input:  []byte("\n"),
			output: []byte("\n"),
		},
		{
			input:  []byte("\n \n"),
			output: []byte("\n \n"),
		},
		{
			input:  []byte("foo bar"),
			output: []byte("foo bar"),
		},
		{
			input:  []byte("foo bar\n"),
			output: []byte("foo bar\n"),
		},
		{
			input:  []byte("line1\nline2"),
			output: []byte("line1\nline2"),
		},
		{
			input:  []byte("line1\nline2\n"),
			output: []byte("line1\nline2\n"),
		},
		{
			input:  []byte("line1\n\nline2\n\n"),
			output: []byte("line1\n\nline2\n\n"),
		},
		{
			input:  []byte("# pint ignore/next-line\nfoo\n"),
			output: []byte("# pint ignore/next-line\n   \n"),
		},
		{
			input:  []byte("# pint ignore/next-line\nfoo"),
			output: []byte("# pint ignore/next-line\n   "),
		},
		{
			input:  []byte("# pint ignore/next-line\nfoo\n\n"),
			output: []byte("# pint ignore/next-line\n   \n\n"),
		},
		{
			input:  []byte("# pint ignore/next-line\nfoo\nbar\n"),
			output: []byte("# pint ignore/next-line\n   \nbar\n"),
		},
		{
			input:  []byte("# pint ignore/next-line  \nfoo\n"),
			output: []byte("# pint ignore/next-line  \n   \n"),
		},
		{
			input:  []byte("#  pint   ignore/next-line  \nfoo\n"),
			output: []byte("#  pint   ignore/next-line  \n   \n"),
		},
		{
			input:  []byte("#pint   ignore/next-line  \nfoo\n"),
			output: []byte("#pint   ignore/next-line  \n   \n"),
		},
		{
			input:  []byte("#pintignore/next-line\nfoo\n"),
			output: []byte("#pintignore/next-line\nfoo\n"),
		},
		{
			input:  []byte("# pint ignore/next-linex\nfoo\n"),
			output: []byte("# pint ignore/next-linex\nfoo\n"),
		},
		{
			input:  []byte("# pint ignore/begin\nfoo\nbar\n"),
			output: []byte("# pint ignore/begin\n   \n   \n"),
		},
		{
			input:  []byte("# pint ignore/begin\nfoo\nbar\n# pint ignore/begin"),
			output: []byte("# pint ignore/begin\n   \n   \n# pint ignore/begin"),
		},
		{
			input:  []byte("# pint ignore/begin\nfoo\nbar\n# pint ignore/begin\nfoo\n"),
			output: []byte("# pint ignore/begin\n   \n   \n# pint ignore/begin\n   \n"),
		},
		{
			input:  []byte("# pint ignore/begin\nfoo\nbar\n# pint ignore/end\nfoo\n"),
			output: []byte("# pint ignore/begin\n   \n   \n# pint ignore/end\nfoo\n"),
		},
		{
			input:  []byte("# pint ignore/begin\nfoo # pint ignore/line\nbar\n# pint ignore/begin"),
			output: []byte("# pint ignore/begin\n    # pint ignore/line\n   \n# pint ignore/begin"),
		},
		{
			input:  []byte("line1\nline2 # pint ignore/line\n"),
			output: []byte("line1\n      # pint ignore/line\n"),
		},
		{
			input:  []byte("line1\nline2 # pint ignore/line\nline3\n"),
			output: []byte("line1\n      # pint ignore/line\nline3\n"),
		},
		{
			input:  []byte("{#- comment #} # pint ignore/line\n"),
			output: []byte(" #- comment #} # pint ignore/line\n"),
		},
		{
			input:  []byte("# pint ignore/file\nfoo\nbar\n# pint ignore/begin\nfoo\n# pint ignore/end\n"),
			output: []byte("# pint ignore/file\n   \n   \n# pint ignore/begin\n   \n# pint ignore/end\n"),
		},
		{
			input:  []byte("foo\n# pint ignore/file\nfoo\nbar\n# pint ignore/begin\nfoo\n# pint ignore/end\n"),
			output: []byte("foo\n# pint ignore/file\n   \n   \n# pint ignore/begin\n   \n# pint ignore/end\n"),
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			r := bytes.NewReader(tc.input)
			output, err := parser.ReadContent(r)

			hadError := err != nil
			if hadError != tc.shouldError {
				t.Errorf("ReadContent() returned err=%v, expected=%v", err, tc.shouldError)
				return
			}

			outputLines := strings.Count(string(output), "\n")
			inputLines := strings.Count(string(tc.input), "\n")
			if outputLines != inputLines {
				t.Errorf("ReadContent() returned %d line(s) while input had %d", outputLines, inputLines)
				return
			}

			require.Equal(t, string(tc.output), string(output), "ReadContent() returned wrong output")
		})
	}
}
