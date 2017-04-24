package input

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInputs(t *testing.T) {
	t.Log("Test - ValidateWithOptions")
	{
		err := ValidateWithOptions("testinput", "tst0", "tst1", "testinput")
		require.NoError(t, err)

		err = ValidateWithOptions("testinput", "test", "input")
		require.Error(t, err)
	}

	t.Log("Test - ValidateIfNotEmpty")
	{
		err := ValidateIfNotEmpty("testinput")
		require.NoError(t, err)

		err = ValidateIfNotEmpty("")
		require.Error(t, err)
	}

	t.Log("Test - SecureInput")
	{
		output := SecureInput("testinput")
		require.Equal(t, "***", output)

		output = SecureInput("")
		require.Equal(t, "", output)
	}

	t.Log("Test - ValidateIfPathExists")
	{
		err := ValidateIfPathExists("/tmp")
		require.NoError(t, err)

		err = ValidateIfPathExists("/not/exists/for/sure")
		require.Error(t, err)
	}
}
