package comments

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseValueUnknownType(t *testing.T) {
	// Test UnknownType case - cannot be reached through normal Parse calls.
	val, err := parseValue(UnknownType, "test", 1)
	require.NoError(t, err)
	require.Nil(t, val)

	// Test InvalidComment case - cannot be reached through normal Parse calls.
	val, err = parseValue(InvalidComment, "test", 1)
	require.NoError(t, err)
	require.Nil(t, val)

	// Test default case - not reachable in practice.
	val, err = parseValue(Type(255), "test", 1)
	require.NoError(t, err)
	require.Nil(t, val)
}
