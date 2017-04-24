package tools

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInputs(t *testing.T) {
	t.Log("Test - IsValueInStringSlice")
	{
		output := IsValueInStringSlice("findme", []string{"val1", "val2", "otherval", "findme", "dontfindme"})
		require.Equal(t, true, output)

		output = IsValueInStringSlice("dontfindme", []string{"val1", "val2", "otherval", "findme"})
		require.Equal(t, false, output)
	}
}
