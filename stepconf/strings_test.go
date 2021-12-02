package stepconf

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_valueString(t *testing.T) {
	var (
		s = "test"
		i = 99
		b = true
	)
	var (
		sPtr = &s
		iPtr = &i
		bPtr = &b
	)
	var (
		sNilPtr *string
		iNilPtr *int64
		bNilPtr *bool
	)

	tests := []struct {
		name string
		v    reflect.Value
		want string
	}{
		{"string", reflect.ValueOf(s), "test"},
		{"string ptr", reflect.ValueOf(sPtr), "test"},
		{"string nil-ptr", reflect.ValueOf(sNilPtr), ""},
		{"int64", reflect.ValueOf(i), "99"},
		{"int64 ptr", reflect.ValueOf(iPtr), "99"},
		{"int64 nil-ptr", reflect.ValueOf(iNilPtr), ""},
		{"bool", reflect.ValueOf(b), "true"},
		{"bool ptr", reflect.ValueOf(bPtr), "true"},
		{"bool nil-ptr", reflect.ValueOf(bNilPtr), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := valueString(tt.v); got != tt.want {
				t.Errorf("valueString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_PrintFormat(t *testing.T) {
	type testConfig struct {
		SimpleString         string `env:"simple_string"`
		FieldWithoutEnvTag   string
		StringThatCanBeEmpty string `env:"string_that_can_be_empty"`
		IntThatCanBeEmpty    int    `env:"int_that_can_be_empty"`
		BoolThatCanBeEmpty   bool   `env:"bool_that_can_be_empty"`
		SensitiveInput       Secret `env:"sensitive_input"`
		ValueOptionInput     string `env:"value_option_input,opt[first,second,third]"`
		RequiredInput        string `env:"required_input,required"`
	}

	cfg := testConfig{
		SimpleString:       "simple value",
		FieldWithoutEnvTag: "This field doesn't have a struct tag",
		// StringThatCanBeEmpty
		// IntThatCanBeEmpty
		// BoolThatCanBeEmpty
		SensitiveInput:   "my secret",
		ValueOptionInput: "second",
		RequiredInput:    "value",
	}

	reader, writer, err := os.Pipe()
	assert.NoError(t, err)

	origStdout := os.Stdout
	os.Stdout = writer

	Print(cfg)

	os.Stdout = origStdout
	assert.NoError(t, writer.Close())

	content, err := ioutil.ReadAll(reader)
	assert.NoError(t, err)

	expected := `[34;1mTestConfig:
[0m- simple_string: simple value
- FieldWithoutEnvTag: This field doesn't have a struct tag
- string_that_can_be_empty: <unset>
- int_that_can_be_empty: <unset>
- bool_that_can_be_empty: <unset>
- sensitive_input: *****
- value_option_input: second
- required_input: value
`
	assert.Equal(t, expected, string(content))
}
