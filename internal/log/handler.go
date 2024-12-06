package log

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"

	"github.com/cloudflare/pint/internal/output"
)

type handler struct {
	dst io.Writer

	escaper *strings.Replacer
	level   slog.Level
	mtx     sync.Mutex
	noColor bool
}

func newHandler(dst io.Writer, level slog.Level, noColor bool) *handler {
	h := handler{
		mtx:     sync.Mutex{},
		dst:     dst,
		level:   level,
		noColor: noColor,
		escaper: strings.NewReplacer(`"`, `\"`),
	}
	return &h
}

func (h *handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *handler) Handle(_ context.Context, record slog.Record) error {
	buf := bytes.NewBuffer(make([]byte, 0, 128))

	var lc output.Color
	switch record.Level {
	case slog.LevelInfo:
		lc = output.White
	case slog.LevelError:
		lc = output.Red
	case slog.LevelWarn:
		lc = output.Yellow
	case slog.LevelDebug:
		lc = output.Magenta
	}
	h.printKey(buf, "level")
	h.printVal(buf, record.Level.String(), lc)
	_, _ = buf.WriteRune(' ')
	h.printKey(buf, "msg")
	h.printVal(buf, record.Message, output.White)

	record.Attrs(func(attr slog.Attr) bool {
		_, _ = buf.WriteRune(' ')
		h.appendAttr(buf, attr)
		return true
	})
	_, _ = buf.WriteRune('\n')

	h.mtx.Lock()
	defer h.mtx.Unlock()

	if _, err := buf.WriteTo(h.dst); err != nil {
		return fmt.Errorf("failed to write buffer: %w", err)
	}

	return nil
}

func (h *handler) WithAttrs(_ []slog.Attr) slog.Handler {
	return h
}

func (h *handler) WithGroup(_ string) slog.Handler {
	return h
}

func (h *handler) printKey(buf *bytes.Buffer, s string) {
	_, _ = buf.WriteString(output.MaybeColor(output.Dim, h.noColor, s+"="))
}

func (h *handler) printVal(buf *bytes.Buffer, s string, color output.Color) {
	if !strings.HasPrefix(s, "[") && !strings.HasPrefix(s, "{") && strings.Contains(s, " ") {
		s = "\"" + h.escaper.Replace(s) + "\""
	}
	_, _ = buf.WriteString(output.MaybeColor(color, h.noColor, s))
}

func (h *handler) appendAttr(buf *bytes.Buffer, attr slog.Attr) {
	attr.Value = attr.Value.Resolve()

	h.printKey(buf, attr.Key)

	// nolint: exhaustive
	switch attr.Value.Kind() {
	case slog.KindAny:
		switch attr.Value.Any().(type) {
		case error:
			h.printVal(buf, formatString(attr), output.Red)
		default:
			h.printVal(buf, formatAny(attr), output.Cyan)
		}
	case slog.KindString:
		h.printVal(buf, formatString(attr), output.Cyan)
	default:
		h.printVal(buf, formatAny(attr), output.Blue)
	}
}

func formatAny(attr slog.Attr) string {
	data, err := json.Marshal(attr.Value.Any())
	if err != nil {
		return attr.Value.String()
	}
	return string(data)
}

func formatString(attr slog.Attr) string {
	return strings.ReplaceAll(attr.Value.String(), "\n", "\\n")
}
