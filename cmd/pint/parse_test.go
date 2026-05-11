package main

import (
	"bytes"
	"testing"

	"github.com/prometheus/prometheus/promql/parser/posrange"
	"github.com/stretchr/testify/require"
)

type unsupportedNode struct{}

func (unsupportedNode) PositionRange() posrange.PositionRange {
	return posrange.PositionRange{}
}

func (unsupportedNode) Pretty(int) string {
	return "unsupported"
}

func (unsupportedNode) String() string {
	return "unsupported"
}

func (unsupportedNode) PromQLExpr() {}

func TestParseUnsupportedNode(t *testing.T) {
	var stdout bytes.Buffer
	parseNode(&stdout, unsupportedNode{}, 0)

	require.Equal(t, "++ node: unsupported\n  ! Unsupported node\n", stdout.String())
}
