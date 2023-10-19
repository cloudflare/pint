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
)

const (
	reset  = "\033[0m"
	dim    = "\033[2m"
	normal = "\u001B[22m"

	fgHiRed     = "\033[91m"
	fgHiYellow  = "\033[93m"
	fgHiBlue    = "\033[94m"
	fgHiMagenta = "\033[95m"
	fgHiCyan    = "\033[96m"
	fgHiWhite   = "\033[97m"
)

type handler struct {
	mtx     *sync.Mutex
	dst     io.Writer
	level   slog.Level
	noColor bool

	escaper *strings.Replacer
}

func newHandler(dst io.Writer, level slog.Level, noColor bool) *handler {
	h := handler{
		mtx:     &sync.Mutex{},
		dst:     dst,
		level:   level,
		noColor: noColor,
	}
	h.escaper = strings.NewReplacer(`"`, `\"`)
	return &h
}

func (h *handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *handler) Handle(_ context.Context, record slog.Record) error {
	buf := bytes.NewBuffer(make([]byte, 0, 256))

	lc := ""
	switch record.Level {
	case slog.LevelInfo:
		lc = fgHiWhite
	case slog.LevelError:
		lc = fgHiRed
	case slog.LevelWarn:
		lc = fgHiYellow
	case slog.LevelDebug:
		lc = fgHiMagenta
	}
	h.printKey(buf, "level")
	h.printVal(buf, record.Level.String(), lc)
	_, _ = buf.WriteString(" ")
	h.printKey(buf, "msg")
	h.printVal(buf, record.Message, fgHiWhite)

	record.Attrs(func(attr slog.Attr) bool {
		_, _ = buf.WriteString(" ")
		h.appendAttr(buf, attr)
		return true
	})
	buf.WriteString("\n")

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
	_, _ = buf.WriteString(h.maybeWriteColor(s+"=", dim))
}

func (h *handler) printVal(buf *bytes.Buffer, s, color string) {
	if !strings.HasPrefix(s, "[") && !strings.HasPrefix(s, "{") && strings.Contains(s, " ") {
		s = "\"" + h.escaper.Replace(s) + "\""
	}
	_, _ = buf.WriteString(h.maybeWriteColor(s, color))
}

func (h *handler) maybeWriteColor(s, color string) string {
	if h.noColor {
		return s
	}
	return fmt.Sprintf("%s%s%s", color, s, reset)
}

func (h *handler) appendAttr(buf *bytes.Buffer, attr slog.Attr) {
	attr.Value = attr.Value.Resolve()

	h.printKey(buf, attr.Key)

	// nolint: exhaustive
	switch attr.Value.Kind() {
	case slog.KindAny:
		switch attr.Value.Any().(type) {
		case error:
			h.printVal(buf, attr.Value.String(), fgHiRed)
		default:
			h.printVal(buf, formatAny(attr), fgHiCyan)
		}
	case slog.KindString:
		h.printVal(buf, attr.Value.String(), fgHiCyan)
	default:
		h.printVal(buf, formatAny(attr), fgHiBlue)
	}
}

func formatAny(attr slog.Attr) string {
	data, err := json.Marshal(attr.Value.Any())
	if err != nil {
		return attr.Value.String()
	}
	return string(data)
}
