package testattachment

import (
	"fmt"
	"testing"

	"github.com/bitrise-io/go-steputils/v2/testreport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testCase(className, name string) testreport.TestCase {
	return testreport.TestCase{ClassName: className, Name: name}
}

func reportOf(testCases ...testreport.TestCase) *testreport.TestReport {
	return &testreport.TestReport{TestSuites: []testreport.TestSuite{{Name: "suite", TestCases: testCases}}}
}

func TestKey(t *testing.T) {
	assert.Equal(t, "com.example.LoginTest__emptyState", Key("com.example.LoginTest", "emptyState"))
	assert.Equal(t, "a_b__c_d_e_f_g_h_i_j", Key(`a"b`, `c*d/e:f<g>h?i\j`))
	assert.Equal(t, "C__x_y", Key("C", "x|y"))
}

func TestMatch(t *testing.T) {
	report := reportOf(
		testCase("com.example.LoginTest", "emptyState"),
		testCase("com.example.LoginTest", "wrongPassword"),
		testCase("com.example.LoginTest", "wrongPassword"),
		testCase("com.example.LoginTest", "wrongPassword__locked"),
		testCase("zold", "alma"),
		testCase("zold", "alma__fa"),
		testCase("C", "foo"),
		testCase("C", "foo__run2"),
		testCase("P", "testFoo[0]"),
	)
	idx := NewIndex(report)

	tests := []struct {
		name      string
		file      string
		wantName  string
		wantRun   int
		wantLabel string
		wantIndex int
		wantErr   error
	}{
		{name: "simple", file: "com.example.LoginTest__emptyState__1.png", wantName: "emptyState", wantLabel: "1"},
		{name: "folder is ignored", file: "some/dir/com.example.LoginTest__emptyState__1.png", wantName: "emptyState", wantLabel: "1"},
		{name: "upper case extension", file: "com.example.LoginTest__emptyState__1.PNG", wantName: "emptyState", wantLabel: "1"},
		{name: "device label", file: "com.example.LoginTest__emptyState__api34_2.mp4", wantName: "emptyState", wantLabel: "api34_2"},
		{name: "no run goes to first occurrence", file: "com.example.LoginTest__wrongPassword__1.png", wantName: "wrongPassword", wantLabel: "1", wantIndex: 1},
		{name: "run selects occurrence", file: "com.example.LoginTest__wrongPassword__run2__api34.png", wantName: "wrongPassword", wantRun: 2, wantLabel: "api34", wantIndex: 2},
		{name: "run beyond occurrences", file: "com.example.LoginTest__wrongPassword__run3__1.png", wantErr: ErrUnknownRun},
		{name: "test name with separator", file: "com.example.LoginTest__wrongPassword__locked__1.png", wantName: "wrongPassword__locked", wantLabel: "1", wantIndex: 3},
		{name: "longer test name wins", file: "zold__alma__fa__1.png", wantName: "alma__fa", wantLabel: "1", wantIndex: 5},
		{name: "shorter test name with label", file: "zold__alma__1.png", wantName: "alma", wantLabel: "1", wantIndex: 4},
		{name: "test name ending in run suffix wins", file: "C__foo__run2__1.png", wantName: "foo__run2", wantLabel: "1", wantIndex: 7},
		{name: "run suffix on other test", file: "C__foo__run1__1.png", wantName: "foo", wantRun: 1, wantLabel: "1", wantIndex: 6},
		{name: "parameterized name", file: "P__testFoo[0]__1.png", wantName: "testFoo[0]", wantLabel: "1", wantIndex: 8},
		{name: "unsupported extension", file: "com.example.LoginTest__emptyState__1.xml", wantErr: ErrUnsupportedType},
		{name: "no separator", file: "debug.log", wantErr: ErrNoConvention},
		{name: "empty label", file: "com.example.LoginTest__emptyState__.png", wantErr: ErrMissingLabel},
		{name: "missing label reads as unknown test", file: "com.example.LoginTest__emptyState.png", wantErr: ErrUnknownTest},
		{name: "unknown test", file: "com.example.LoginTest__missing__1.png", wantErr: ErrUnknownTest},
		{name: "prefix only is not enough", file: "C__foo__bar__extra__1.png", wantErr: ErrUnknownTest},
		{name: "run zero", file: "com.example.LoginTest__wrongPassword__run0__1.png", wantErr: ErrUnknownTest},
		{name: "run not a number", file: "com.example.LoginTest__wrongPassword__runx__1.png", wantErr: ErrUnknownTest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, err := idx.Match(tt.file)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, match.TestCase.Name)
			assert.Equal(t, tt.wantRun, match.Run)
			assert.Equal(t, tt.wantLabel, match.Label)
			assert.Same(t, &report.TestSuites[0].TestCases[tt.wantIndex], match.TestCase)
		})
	}
}

func TestNewIndex_ambiguousKeys(t *testing.T) {
	idx := NewIndex(reportOf(
		testCase("C", "a:b"),
		testCase("C", "a/b"),
		testCase("C", "ok"),
	))

	assert.Equal(t, []string{"C__a_b"}, idx.Ambiguous())

	_, err := idx.Match("C__a_b__1.png")
	require.ErrorIs(t, err, ErrAmbiguousTest)

	match, err := idx.Match("C__ok__1.png")
	require.NoError(t, err)
	assert.Equal(t, "ok", match.TestCase.Name)
}

func TestNewIndex_retriesAreNotAmbiguous(t *testing.T) {
	idx := NewIndex(reportOf(testCase("C", "flaky"), testCase("C", "flaky")))

	assert.Empty(t, idx.Ambiguous())
}

func TestNewIndex_nestedSuites(t *testing.T) {
	report := &testreport.TestReport{TestSuites: []testreport.TestSuite{{
		Name: "outer",
		TestSuites: []testreport.TestSuite{{
			Name:      "inner",
			TestCases: []testreport.TestCase{testCase("C", "nested")},
		}},
	}}}
	idx := NewIndex(report)

	match, err := idx.Match("C__nested__1.png")
	require.NoError(t, err)
	assert.Same(t, &report.TestSuites[0].TestSuites[0].TestCases[0], match.TestCase)
}

func BenchmarkMatch(b *testing.B) {
	testCases := make([]testreport.TestCase, 50_000)
	for i := range testCases {
		testCases[i] = testCase(fmt.Sprintf("com.example.feature%d.SomeScreenTest", i%500), fmt.Sprintf("givenState%dWhenActionThenResult", i))
	}
	idx := NewIndex(reportOf(testCases...))

	files := make([]string, 5_000)
	for i := range files {
		tc := testCases[i*10]
		files[i] = Key(tc.ClassName, tc.Name) + "__1.png"
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, file := range files {
			if _, err := idx.Match(file); err != nil {
				b.Fatal(err)
			}
		}
	}
}
