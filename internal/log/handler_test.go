package log

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHandler(t *testing.T) {
	type testCaseT struct {
		run      func(l *slog.Logger)
		expected string
		noColor  bool
	}

	testCases := []testCaseT{
		{
			noColor: true,
			run: func(l *slog.Logger) {
				l.Debug("foo", slog.Int("count", 5))
			},
			expected: "level=DEBUG msg=foo count=5\n",
		},
		{
			noColor: false,
			run: func(l *slog.Logger) {
				l.Debug("foo", slog.Int("count", 5))
			},
			expected: "[2mlevel=[0m[95mDEBUG[0m [2mmsg=[0m[97mfoo[0m [2mcount=[0m[94m5[0m\n", // nolint: staticcheck
		},
		{
			noColor: true,
			run: func(l *slog.Logger) {
				l.Debug("foo", slog.Int("count", 5))
				l.Info("bar", slog.String("string", "a b c"), slog.Any("list", []int{1, 2, 3}))
			},
			expected: "level=DEBUG msg=foo count=5\nlevel=INFO msg=bar string=\"a b c\" list=[1,2,3]\n",
		},
		{
			noColor: true,
			run: func(l *slog.Logger) {
				l.Warn("bar", slog.Any("strings", []string{"a", "b", "c + d"}))
			},
			expected: "level=WARN msg=bar strings=[\"a\",\"b\",\"c + d\"]\n",
		},
		{
			noColor: true,
			run: func(l *slog.Logger) {
				l.Error("bar", slog.Any("err", errors.New("error")))
			},
			expected: "level=ERROR msg=bar err=error\n",
		},
		{
			noColor: false,
			run: func(l *slog.Logger) {
				l.Error("bar", slog.Any("err", errors.New("error")))
			},
			expected: "[2mlevel=[0m[91mERROR[0m [2mmsg=[0m[97mbar[0m [2merr=[0m[91merror[0m\n", // nolint: staticcheck
		},
		{
			noColor: true,
			run: func(l *slog.Logger) {
				l.Error("bar", slog.Any("err", nil))
			},
			expected: "level=ERROR msg=bar err=null\n",
		},
		{
			noColor: true,
			run: func(l *slog.Logger) {
				type Foo struct {
					N json.Number
				}
				x := Foo{json.Number(`invalid`)}
				l.Error("bar", slog.Any("err", x))
			},
			expected: "level=ERROR msg=bar err={invalid}\n",
		},
		{
			noColor: true,
			run: func(l *slog.Logger) {
				l.With(slog.String("with", "true")).Error("bar")
			},
			expected: "level=ERROR msg=bar\n",
		},
		{
			noColor: true,
			run: func(l *slog.Logger) {
				l.Info("bar", slog.Group("group", slog.String("with", "true")))
			},
			expected: "level=INFO msg=bar group=[{\"Key\":\"with\",\"Value\":{}}]\n",
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			dst := bytes.NewBufferString("")
			tc.run(slog.New(newHandler(dst, slog.LevelDebug.Level(), tc.noColor)))
			require.Equal(t, tc.expected, dst.String())
		})
	}
}
