package stepconf

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/bitrise-io/go-steputils/v2/stepconf/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var invalid = map[string]string{
	"name":               "Invalid config",
	"build_number":       "notnumber",
	"large_build_number": "notnumber",
	"is_update":          "notbool",
	"items":              "one,two,three",
	"password":           "pass1234",
	"empty":              "",
	"missing":            "",
	"file":               "/tmp/not-exist",
	"dir":                "/etc/hosts",
	"export_method":      "four",
	"myfloat64":          "asd.asd",
}

var valid = map[string]string{
	"name":               "Example",
	"build_number":       "11",
	"large_build_number": "2402097469",
	"is_update":          "yes",
	"items":              "item1|item2|item3",
	"multiline_items": `item1
item2
item3`,
	"password":        "pass1234",
	"empty":           "",
	"mandatory":       "present",
	"file":            "/etc/hosts",
	"dir":             "/tmp",
	"export_method":   "dev",
	"emptyptrstr":     "",
	"ptrstr":          "test",
	"emptyptrint":     "",
	"ptrint":          "5",
	"myfloat64":       "0.3",
	"mySecondFloat64": "0.3\t ",
}

type Config struct {
	Name             string   `env:"name"`
	BuildNumber      int      `env:"build_number"`
	LargeBuildNumber int64    `env:"large_build_number"`
	IsUpdate         bool     `env:"is_update"`
	Testfloat64      float64  `env:"myfloat64"`
	MySecondFloat64  float64  `env:"mySecondFloat64"`
	Items            []string `env:"items"`
	MultilineItems   []string `env:"multiline_items,multiline"`
	Password         Secret   `env:"password"`
	Empty            string   `env:"empty"`
	Mandatory        string   `env:"mandatory,required"`
	TempFile         string   `env:"file,file"`
	TempDir          string   `env:"dir,dir"`
	ExportMethod     string   `env:"export_method,opt[dev,qa,prod]"`
	EmptyPtrStr      *string  `env:"emptyptrstr"`
	PtrStr           *string  `env:"ptrstr"`
	EmptyPtrInt      *int     `env:"emptyptrint"`
	PtrInt           *int     `env:"ptrint"`
}

func TestParse(t *testing.T) {
	var c Config

	envGetter := new(mocks.Repository)
	for key, value := range valid {
		envGetter.On("Get", key).Return(value)
	}

	if err := parse(&c, envGetter); err != nil {
		t.Error(err.Error())
	}

	if c.Name != "Example" {
		t.Errorf("expected %s, got %v", "Example", c.Name)
	}
	if c.BuildNumber != 11 {
		t.Errorf("expected %d, got %v", 11, c.BuildNumber)
	}
	if c.LargeBuildNumber != 2402097469 {
		t.Errorf("expected %d, got %v", 2402097469, c.LargeBuildNumber)
	}
	if !c.IsUpdate {
		t.Errorf("expected %t, got %v", true, c.IsUpdate)
	}
	if len(c.Items) != 3 ||
		c.Items[0] != "item1" ||
		c.Items[1] != "item2" ||
		c.Items[2] != "item3" {
		t.Errorf("expected %#v, got %#v", []string{"item1", "item2", "item3"}, c.Items)
	}
	if len(c.MultilineItems) != 3 ||
		c.Items[0] != "item1" ||
		c.Items[1] != "item2" ||
		c.Items[2] != "item3" {
		t.Errorf("expected %#v, got %#v", []string{"item1", "item2", "item3"}, c.Items)
	}
	if c.Password != "pass1234" {
		t.Errorf("expected %s, got %v", "pass1234", c.Password)
	}
	if c.Empty != "" {
		t.Errorf("expected %s, got %v", "", c.Empty)
	}
	if c.Mandatory != "present" {
		t.Errorf("expected %s, got %v", "present", c.Mandatory)
	}
	if c.TempFile != "/etc/hosts" {
		t.Errorf("expected %s, got %v", "/etc/hosts", c.TempFile)
	}
	if c.TempDir != "/tmp" {
		t.Errorf("expected %s, got %v", "/tmp", c.TempDir)
	}
	if c.ExportMethod != "dev" {
		t.Errorf("expected %s, got %v", "dev", c.ExportMethod)
	}
	if c.EmptyPtrStr != nil {
		t.Errorf("expected %s, got %v", "nil", c.ExportMethod)
	}
	if c.PtrStr == nil || *c.PtrStr != "test" {
		t.Errorf("expected %s, got %v", "test", c.ExportMethod)
	}
	if c.EmptyPtrInt != nil {
		t.Errorf("expected %s, got %v", "nil", c.ExportMethod)
	}
	if c.PtrInt == nil || *c.PtrInt != 5 {
		t.Errorf("expected %s, got %v", "test", c.ExportMethod)
	}
}

func TestNotPointer(t *testing.T) {
	var c Config
	if err := parse(c, nil); err == nil {
		t.Error("no failure when input parameter is a pointer")
	}
}

func TestNotStruct(t *testing.T) {
	var basicType string
	if err := parse(&basicType, nil); err == nil {
		t.Error("no failure when input parameter is not a struct")
	}
}

func TestInvalidEnvs(t *testing.T) {
	var c Config

	envGetter := new(mocks.Repository)
	for key, value := range invalid {
		envGetter.On("Get", key).Return(value)
	}
	envGetter.On("Get", mock.Anything).Return("")

	if err := parse(&c, envGetter); err == nil {
		t.Error("no failure when invalid values used")
	}
}

func TestValidateNotExists(t *testing.T) {
	var c struct {
		Length string `env:"length,length"`
	}

	envGetter := new(mocks.Repository)
	envGetter.On("Get", mock.Anything).Return("")

	if err := parse(&c, envGetter); err == nil {
		t.Error("no failure when validate tag is not exists")
	}
}

func TestRequired(t *testing.T) {
	var c struct {
		Required string `env:"required,required"`
	}

	envGetter := new(mocks.Repository)
	envGetter.On("Get", mock.Anything).Return("")

	if err := parse(&c, envGetter); err == nil {
		t.Error("no failure when required env var is missing")
	}

	envGetter = new(mocks.Repository)
	envGetter.On("Get", "required").Return("set")

	if err := parse(&c, envGetter); err != nil {
		t.Error("failure when required env var is set")
	}
}

func TestValidatePath(t *testing.T) {
	var c struct {
		Path string `env:"path,file"`
	}

	envGetter := new(mocks.Repository)
	envGetter.On("Get", "path").Return("/not/exist")

	if err := parse(&c, envGetter); err == nil {
		t.Error("no failure when path does not exist")
	}

	f, err := os.CreateTemp("", "stepconf_test")
	if err != nil {
		t.Fatalf("should not have error: %s", err)
	}

	envGetter = new(mocks.Repository)
	envGetter.On("Get", "path").Return(f.Name())
	if err := parse(&c, envGetter); err != nil {
		t.Error("failure when path is exist")
	}
}

func TestValidateDir(t *testing.T) {
	var c struct {
		Dir string `env:"dir,dir"`
	}

	envGetter := new(mocks.Repository)
	envGetter.On("Get", "dir").Return("/not/exist")

	if err := parse(&c, envGetter); err == nil {
		t.Error("no failure when dir does not exist")
	}

	dir, err := os.MkdirTemp("", "stepconf_test")
	if err != nil {
		t.Fatalf("should not have error: %s", err)
	}

	envGetter = new(mocks.Repository)
	envGetter.On("Get", "dir").Return(dir)

	if err := parse(&c, envGetter); err != nil {
		t.Error("failure when dir does exist")
	}
}

func TestValueOptions(t *testing.T) {
	var c struct {
		Option string `env:"option,opt[opt1,opt2,opt3]"`
	}

	envGetter := new(mocks.Repository)
	envGetter.On("Get", "option").Return("no-opt")

	if err := parse(&c, envGetter); err == nil {
		t.Error("no failure when value is not in value options")
	}

	envGetter = new(mocks.Repository)
	envGetter.On("Get", "option").Return("opt1")

	if err := parse(&c, envGetter); err != nil {
		t.Error("failure when value is in value options")
	}
}

func TestValueOptionsWithComma(t *testing.T) {
	var c struct {
		Option string `env:"option,opt[opt1,opt2,'opt1,opt2']"`
	}

	envGetter := new(mocks.Repository)
	envGetter.On("Get", "option").Return("opt1,opt2")

	if err := parse(&c, envGetter); err != nil {
		t.Errorf("failure when value is in value options: %s", err)
	}
	if c.Option != "opt1,opt2" {
		t.Errorf("expected %s, got %v", "opt1", c.Option)
	}

	envGetter = new(mocks.Repository)
	envGetter.On("Get", "option").Return("")

	if err := parse(&c, envGetter); err == nil {
		t.Errorf("no failure when value is not in value options")
	}
}

func Test_GetRangeValues(t *testing.T) {
	tests := []struct {
		value     string
		name      string
		wantMin   string
		wantMax   string
		wantMinBr string
		wantMaxBr string
		wantErr   bool
	}{
		{"range[6..]", "MinIntPositive", "6", "", "[", "]", false},
		{"range[-6..]", "MinIntNegative", "-6", "", "[", "]", false},
		{"range[3.14..]", "MinDoublePositive", "3.14", "", "[", "]", false},
		{"range[-3.14..]", "MinDoubleNegative", "-3.14", "", "[", "]", false},

		{"range[..6]", "MaxIntPositive", "", "6", "[", "]", false},
		{"range[..-6]", "MaxIntNegative", "", "-6", "[", "]", false},
		{"range[..3.14]", "MaxDoublePositive", "", "3.14", "[", "]", false},
		{"range[..-3.14]", "MaxDoubleNegative", "", "-3.14", "[", "]", false},

		{"range[3..6]", "MinMaxIntInt", "3", "6", "[", "]", false},
		{"range[3..6.0]", "MinMaxIntDouble", "3", "6.0", "[", "]", false},
		{"range[3.14..6]", "MinMaxDoubleInt", "3.14", "6", "[", "]", false},
		{"range[3.14..6.0]", "MinMaxFloatDouble", "3.14", "6.0", "[", "]", false},

		{"invalid", "Invalid1", "", "", "", "", true},
		{"range[..]", "Invalid2", "", "", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMin, gotMax, gotMinBr, gotMaxBr, err := getRangeValues(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetRangeValues() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotMin != tt.wantMin {
				t.Errorf("GetRangeValues() gotMin = %v, want %v", gotMin, tt.wantMin)
			}
			if gotMax != tt.wantMax {
				t.Errorf("GetRangeValues() gotMax = %v, want %v", gotMax, tt.wantMax)
			}
			if gotMinBr != tt.wantMinBr {
				t.Errorf("GetRangeValues() gotMinBr = %v, want %v", gotMinBr, tt.wantMinBr)
			}
			if gotMaxBr != tt.wantMaxBr {
				t.Errorf("GetRangeValues() gotMaxBr = %v, want %v", gotMaxBr, tt.wantMaxBr)
			}
		})
	}
}

func Test_ValidateRangeFields(t *testing.T) {
	tests := []struct {
		name       string
		valueStr   string
		constraint string
		wantErr    bool
	}{
		{"ValidIntIntInclIncl1", "3", "range[3..8]", false},
		{"ValidIntIntInclIncl2", "5", "range[3..8]", false},
		{"ValidIntIntInclIncl3", "8", "range[3..8]", false},

		{"ValidIntIntExclIncl1", "3", "range]3..8]", true},
		{"ValidIntIntExclIncl2", "5", "range]3..8]", false},
		{"ValidIntIntExclIncl3", "8", "range]3..8]", false},

		{"ValidIntIntInclExcl1", "3", "range[3..8[", false},
		{"ValidIntIntInclExcl2", "5", "range[3..8[", false},
		{"ValidIntIntInclExcl3", "8", "range[3..8[", true},

		{"ValidIntIntExclExcl1", "3", "range]3..8[", true},
		{"ValidIntIntExclExcl2", "5", "range]3..8[", false},
		{"ValidIntIntExclExcl3", "8", "range]3..8[", true},

		{"ValidDoubleDoubleInclIncl", "3.14", "range[3.14..8.5]", false},
		{"ValidDoubleDoubleInclIncl2", "5.0", "range[3.14..8.5]", false},
		{"ValidDoubleDoubleInclIncl3", "8.5", "range[3.14..8.5]", false},

		{"ValidDoubleDoubleExclIncl1", "3.14", "range]3.14..8.5]", true},
		{"ValidDoubleDoubleExclIncl2", "5.0", "range]3.14..8.5]", false},
		{"ValidDoubleDoubleExclIncl3", "8.5", "range]3.14..8.5]", false},

		{"ValidDoubleDoubleInclExcl1", "3.14", "range[3.14..8.5[", false},
		{"ValidDoubleDoubleInclExcl2", "5.0", "range[3.14..8.5[", false},
		{"ValidDoubleDoubleInclExcl3", "8.5", "range[3.14..8.5[", true},

		{"ValidDoubleDoubleExclExcl1", "3.14", "range]3.14..8.5[", true},
		{"ValidDoubleDoubleExclExcl2", "5.0", "range]3.14..8.5[", false},
		{"ValidDoubleDoubleExclExcl3", "8.5", "range]3.14..8.5[", true},

		{"InvalidCombination1", "3", "range[1..5.5]", true},
		{"InvalidCombination2", "3", "range[1.0..5.5]", true},
		{"InvalidCombination3", "3.14", "range[3.14..8]", true},
		{"InvalidCombination4", "3.14", "range[3..8.5]", true},

		{"InvalidRange", "5", "range[9..8]", true},
		{"InvalidValue1", "15", "range[4..8]", true},
		{"InvalidValue2", "5", "range[5..5]", true},

		{"OptionalValue", "", "range[5..6]", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateRangeFields(tt.valueStr, tt.constraint); (err != nil) != tt.wantErr {
				t.Errorf("ValidateRangeFields() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseErrorFormatOfNonSecretFields(t *testing.T) {
	tests := []struct {
		name    string
		conf    interface{}
		value   string
		wantErr string
	}{
		{
			name: "value options",
			conf: &struct {
				Input string `env:"input,opt[dev,qa]"`
			}{},
			value:   "prod",
			wantErr: "- Input: prod: value is not in value options (opt[dev,qa])\n",
		},
		{
			name: "file",
			conf: &struct {
				Input string `env:"input,file"`
			}{},
			value:   "/not/exist",
			wantErr: "- Input: /not/exist: file does not exist\n",
		},
		{
			name: "range, not a number",
			conf: &struct {
				Input string `env:"input,range[1..5]"`
			}{},
			value:   "abc",
			wantErr: "- Input: abc: failed to parse value abc: value abc is could not be parsed\n",
		},
		{
			name: "range, out of range",
			conf: &struct {
				Input float64 `env:"input,range[1.5..5.5]"`
			}{},
			value:   "9.5",
			wantErr: "- Input: 9.5: value 9.500000 is out of range 1.500000-5.500000\n",
		},
		{
			name: "required",
			conf: &struct {
				Input string `env:"input,required"`
			}{},
			value:   "",
			wantErr: "- Input: required variable is not present\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envGetter := new(mocks.Repository)
			envGetter.On("Get", "input").Return(tt.value)

			err := parse(tt.conf, envGetter)

			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestParseErrorOfSecretFieldsIsRedacted(t *testing.T) {
	existingFile := filepath.Join(t.TempDir(), "s3cr3t-file")
	require.NoError(t, os.WriteFile(existingFile, nil, 0600))

	tests := []struct {
		name    string
		conf    interface{}
		value   string
		wantErr string
	}{
		{
			name: "value options",
			conf: &struct {
				Token Secret `env:"token,opt[dev,qa]"`
			}{},
			value:   "s3cr3t",
			wantErr: "value is not in value options (opt[dev,qa])",
		},
		{
			name: "value options, pointer",
			conf: &struct {
				Token *Secret `env:"token,opt[dev,qa]"`
			}{},
			value:   "s3cr3t",
			wantErr: "value is not in value options (opt[dev,qa])",
		},
		{
			name: "file",
			conf: &struct {
				Token Secret `env:"token,file"`
			}{},
			value:   "/s3cr3t/not/exist",
			wantErr: "file does not exist",
		},
		{
			name: "dir, does not exist",
			conf: &struct {
				Token Secret `env:"token,dir"`
			}{},
			value:   "/s3cr3t/not/exist",
			wantErr: "file does not exist",
		},
		{
			name: "dir, not a directory",
			conf: &struct {
				Token Secret `env:"token,dir"`
			}{},
			value:   existingFile,
			wantErr: "not a directory",
		},
		{
			name: "range, not a number",
			conf: &struct {
				Token Secret `env:"token,range[1..5]"`
			}{},
			value:   "s3cr3t",
			wantErr: "value is invalid or out of range (range[1..5])",
		},
		{
			name: "range, out of range",
			conf: &struct {
				Token Secret `env:"token,range[1..5]"`
			}{},
			value:   "8675309",
			wantErr: "value is invalid or out of range (range[1..5])",
		},
		{
			name: "range, out of range, pointer",
			conf: &struct {
				Token *Secret `env:"token,range[1.5..5.5]"`
			}{},
			value:   "8675.309",
			wantErr: "value is invalid or out of range (range[1.5..5.5])",
		},
		{
			name: "invalid constraint",
			conf: &struct {
				Token Secret `env:"token,nosuch"`
			}{},
			value:   "s3cr3t",
			wantErr: "invalid constraint (nosuch)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envGetter := new(mocks.Repository)
			envGetter.On("Get", "token").Return(tt.value)

			err := parse(tt.conf, envGetter)

			require.Error(t, err)
			require.NotContains(t, err.Error(), tt.value)
			require.Contains(t, err.Error(), "- Token: *****: "+tt.wantErr+"\n")

			// parse flattens the field errors into a string, so check the exported fields separately.
			field := reflect.TypeOf(tt.conf).Elem().Field(0)
			_, constraint := parseTag(field.Tag.Get("env"))
			parseErr := newParseError(field, tt.value, validateConstraint(tt.value, constraint))

			require.Equal(t, "*****", parseErr.Value)
			require.NotContains(t, parseErr.Err.Error(), tt.value)
			require.Equal(t, "Token: *****: "+tt.wantErr, parseErr.Error())
		})
	}
}

func TestParseErrorOfEmptySecretField(t *testing.T) {
	var c struct {
		Token Secret `env:"token,required"`
	}

	envGetter := new(mocks.Repository)
	envGetter.On("Get", "token").Return("")

	err := parse(&c, envGetter)

	require.Error(t, err)
	require.Contains(t, err.Error(), "- Token: required variable is not present\n")
}
