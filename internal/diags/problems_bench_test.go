package diags

import (
	"strings"
	"testing"

	"github.com/cloudflare/pint/internal/output"
)

// BenchmarkInjectDiagnosticsLongExpr measures diagnostic injection for a
// long single-line PromQL expression that triggers AST trimming.
func BenchmarkInjectDiagnosticsLongExpr(b *testing.B) {
	// 180-byte PromQL line that would need trimming.
	input := "expr: sum by (instance) (rate(http_requests_total{job=\"api\",status=~\"5..\"}[5m])) / sum by (instance) (rate(up{job=\"api\"}[5m])) > 0.01\n"
	lines := strings.Split(input, "\n")
	pos := PositionRanges{{Line: 1, FirstColumn: 7, LastColumn: len(lines[0]) - 1}}
	// 4 diagnostics inside the expression.
	diags := []Diagnostic{
		{Message: "dead code", Pos: pos, FirstColumn: 10, LastColumn: 20},
		{Message: "dead code", Pos: pos, FirstColumn: 40, LastColumn: 60},
		{Message: "dead code", Pos: pos, FirstColumn: 80, LastColumn: 100},
		{Message: "dead code", Pos: pos, FirstColumn: 120, LastColumn: 140},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = InjectDiagnostics(input, diags, output.None)
	}
}

// BenchmarkInjectDiagnosticsShortExpr measures the fast path where no trimming
// is needed.
func BenchmarkInjectDiagnosticsShortExpr(b *testing.B) {
	input := "expr: up == 0\n"
	lines := strings.Split(input, "\n")
	pos := PositionRanges{{Line: 1, FirstColumn: 7, LastColumn: len(lines[0]) - 1}}
	diags := []Diagnostic{
		{Message: "dead code", Pos: pos, FirstColumn: 7, LastColumn: 9},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = InjectDiagnostics(input, diags, output.None)
	}
}
