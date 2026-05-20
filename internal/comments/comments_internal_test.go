package comments

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/diags"
)

func TestParseValueUnknownType(t *testing.T) {
	// Test UnknownType case - cannot be reached through normal Parse calls.
	pos := diags.PositionRanges{{Line: 1, FirstColumn: 1, LastColumn: 4}}
	val, err := parseValue(UnknownType, "test", pos, 0)
	require.NoError(t, err)
	require.Nil(t, val)

	// Test InvalidComment case - cannot be reached through normal Parse calls.
	val, err = parseValue(InvalidComment, "test", pos, 0)
	require.NoError(t, err)
	require.Nil(t, val)

	// Test default case - not reachable in practice.
	val, err = parseValue(Type(255), "test", pos, 0)
	require.NoError(t, err)
	require.Nil(t, val)
}
