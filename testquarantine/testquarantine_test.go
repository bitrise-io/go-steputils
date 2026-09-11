package testquarantine_test

import (
	"testing"

	"github.com/bitrise-io/go-steputils/v2/testquarantine"
	"github.com/stretchr/testify/require"
)

func TestParseQuarantinedTests(t *testing.T) {
	jsonContent := `[
		{
			"testSuiteName": ["BullsEyeTests", "BullsEyeSwiftTestingTests"],
			"className": "BullsEyeSwiftTestingTests",
			"testCaseName": "Score is computed when the guess matches the target",
			"testCaseIdentifier": "BullsEyeSwiftTestingTests/scoreIsComputedWhenGuessMatchesTarget()"
		},
		{
			"testSuiteName": ["BullsEyeTests"],
			"className": "BullsEyeTests",
			"testCaseName": "testGameStyleSwitch"
		}
	]`

	quarantinedTests, err := testquarantine.ParseQuarantinedTests(jsonContent)

	require.NoError(t, err)
	require.Equal(t, []testquarantine.QuarantinedTest{
		{
			TestSuiteName:      []string{"BullsEyeTests", "BullsEyeSwiftTestingTests"},
			ClassName:          "BullsEyeSwiftTestingTests",
			TestCaseName:       "Score is computed when the guess matches the target",
			TestCaseIdentifier: "BullsEyeSwiftTestingTests/scoreIsComputedWhenGuessMatchesTarget()",
		},
		{
			TestSuiteName: []string{"BullsEyeTests"},
			ClassName:     "BullsEyeTests",
			TestCaseName:  "testGameStyleSwitch",
		},
	}, quarantinedTests)
}

func TestParseQuarantinedTests_emptyContent(t *testing.T) {
	quarantinedTests, err := testquarantine.ParseQuarantinedTests("")

	require.NoError(t, err)
	require.Nil(t, quarantinedTests)
}

func TestParseQuarantinedTests_invalidJSON(t *testing.T) {
	_, err := testquarantine.ParseQuarantinedTests("not json")

	require.Error(t, err)
}
