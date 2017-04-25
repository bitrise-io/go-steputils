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
		require.EqualError(t, err, "invalid parameter: testinput, available: [test input]")

		err = ValidateWithOptions("testinput")
		require.EqualError(t, err, "invalid parameter: testinput, available: []")
	}

	t.Log("Test - ValidateIfNotEmpty")
	{
		err := ValidateIfNotEmpty("testinput")
		require.NoError(t, err)

		err = ValidateIfNotEmpty("")
		require.EqualError(t, err, "parameter not specified")
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
		require.EqualError(t, err, "path not exist at: /not/exists/for/sure")
	}
}
