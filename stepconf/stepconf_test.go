package stepconf_test

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/bitrise-io/go-steputils/stepconf"
)

var invalid = map[string]string{
	"name":          "Invalid config",
	"build_number":  "notnumber",
	"is_update":     "notbool",
	"items":         "one,two,three",
	"password":      "pass1234",
	"empty":         "",
	"missing":       "",
	"file":          "/tmp/not-exist",
	"dir":           "/etc/hosts",
	"export_method": "four",
}

var valid = map[string]string{
	"name":          "Example",
	"build_number":  "11",
	"is_update":     "yes",
	"items":         "item1|item2|item3",
	"password":      "pass1234",
	"empty":         "",
	"mandatory":     "present",
	"file":          "/etc/hosts",
	"dir":           "/tmp",
	"export_method": "dev",
}

func setEnvironment(envs map[string]string) {
	os.Clearenv()
	for env, value := range envs {
		err := os.Setenv(env, value)
		if err != nil {
			log.Fatal()
		}
	}
}

type Config struct {
	Name         string          `env:"name"`
	BuildNumber  int             `env:"build_number"`
	IsUpdate     bool            `env:"is_update"`
	Items        []string        `env:"items"`
	Password     stepconf.Secret `env:"password"`
	Empty        string          `env:"empty"`
	Mandatory    string          `env:"mandatory,required"`
	TempFile     string          `env:"file,file"`
	TempDir      string          `env:"dir,dir"`
	ExportMethod string          `env:"export_method,opt[dev,qa,prod]"`
}

func TestParse(t *testing.T) {
	var c Config
	os.Clearenv()
	setEnvironment(valid)

	err := stepconf.Parse(&c)
	if err != nil {
		t.Error(err.Error())
	}
	if c.Name != "Example" {
		t.Errorf("expected %s, got %v", "Example", c.Name)
	}
	if c.BuildNumber != 11 {
		t.Errorf("expected %d, got %v", 11, c.BuildNumber)
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
}

func TestNotPointer(t *testing.T) {
	var c Config
	if err := stepconf.Parse(c); err == nil {
		t.Error("no failure when input parameter is a pointer")
	}
}

func TestNotStruct(t *testing.T) {
	var basicType string
	if err := stepconf.Parse(&basicType); err == nil {
		t.Error("no failure when input parameter is not a struct")
	}
}

func TestInvalidEnvs(t *testing.T) {
	setEnvironment(invalid)
	var c Config
	if err := stepconf.Parse(&c); err == nil {
		t.Error("no failure when invalid values used")
	}
}

func TestValidateNotExists(t *testing.T) {
	type invalid struct {
		Length string `env:"length,length"`
	}
	var c invalid
	if err := stepconf.Parse(&c); err == nil {
		t.Error("no failure when validate tag is not exists")
	}
}

func TestRequired(t *testing.T) {
	type config struct {
		Required string `env:"required,required"`
	}
	var c config
	os.Clearenv()

	if err := stepconf.Parse(&c); err == nil {
		t.Error("no failure when required env var is missing")
	}

	err := os.Setenv("required", "set")
	if err != nil {
		t.Fatalf("should not have error: %s", err)
	}
	if err := stepconf.Parse(&c); err != nil {
		t.Error("failure when required env var is set")
	}
}

func TestValidatePath(t *testing.T) {
	type config struct {
		Path string `env:"path,file"`
	}
	var c config
	os.Clearenv()

	if err := os.Setenv("path", "/not/exist"); err != nil {
		t.Fatalf("should not have error: %s", err)
	}
	if err := stepconf.Parse(&c); err == nil {
		t.Error("no failure when path does not exist")
	}

	f, err := ioutil.TempFile("", "stepconf_test")
	if err != nil {
		t.Fatalf("should not have error: %s", err)
	}
	if err := os.Setenv("path", f.Name()); err != nil {
		t.Fatalf("should not have error: %s", err)
	}
	if err := stepconf.Parse(&c); err != nil {
		t.Error("failure when path is exist")
	}
}

func TestValidateDir(t *testing.T) {
	type config struct {
		Dir string `env:"dir,dir"`
	}
	var c config
	os.Clearenv()

	if err := os.Setenv("dir", "/not/exist"); err != nil {
		t.Fatalf("should not have error: %s", err)
	}
	if err := stepconf.Parse(&c); err == nil {
		t.Error("no failure when dir does not exist")
	}

	dir, err := ioutil.TempDir("", "stepconf_test")
	if err != nil {
		t.Fatalf("should not have error: %s", err)
	}
	if err := os.Setenv("dir", dir); err != nil {
		t.Fatalf("should not have error: %s", err)
	}
	if err := stepconf.Parse(&c); err != nil {
		t.Error("failure when dir does exist")
	}
}

func TestValueOptions(t *testing.T) {
	type config struct {
		Option string `env:"option,opt[opt1,opt2,opt3]"`
	}
	var c config
	os.Clearenv()

	if err := os.Setenv("option", "no-opt"); err != nil {
		t.Fatalf("should not have error: %s", err)
	}
	if err := stepconf.Parse(&c); err == nil {
		t.Error("no failure when value is not in value options")
	}

	if err := os.Setenv("option", "opt1"); err != nil {
		t.Fatalf("should not have error: %s", err)
	}
	if err := stepconf.Parse(&c); err != nil {
		t.Error("failure when value is in value options")
	}
}

func TestValueOptionsWithComma(t *testing.T) {
	type config struct {
		Option string `env:"option,opt[opt1,opt2,'opt1,opt2']"`
	}
	var c config
	os.Clearenv()
	if err := os.Setenv("option", "opt1,opt2"); err != nil {
		t.Fatalf("should not have error: %s", err)
	}
	if err := stepconf.Parse(&c); err != nil {
		t.Errorf("failure when value is in value options: %s", err)
	}
	if c.Option != "opt1,opt2" {
		t.Errorf("expected %s, got %v", "opt1", c.Option)
	}
	if err := os.Setenv("option", ""); err != nil {
		t.Fatalf("should not have error: %s", err)
	}
	if err := stepconf.Parse(&c); err == nil {
		t.Errorf("no failure when value is not in value options")
	}
}

func ExampleParse() {
	c := struct {
		Name string `env:"ENV_NAME"`
		Num  int    `env:"ENV_NUMBER"`
	}{}
	if err := os.Setenv("ENV_NAME", "example"); err != nil {
		panic(err)
	}
	if err := os.Setenv("ENV_NUMBER", "1548"); err != nil {
		panic(err)
	}
	if err := stepconf.Parse(&c); err != nil {
		log.Fatal(err)
	}
	fmt.Println(c)
	// Output: {example 1548}
}

func Test_getMinMaxValue(t *testing.T) {
	tests := []struct {
		value   string
		name    string
		wantMin string
		wantMax string
		wantErr bool
	}{
		{"min=6", "MinIntPositive", "6", "", false},
		{"min=-6", "MinIntNegative", "-6", "", false},
		{"min=3.0", "MinFloatPositive", "3.0", "", false},
		{"min=-3.0", "MinFloatNegative", "-3.0", "", false},

		{"max=6", "MaxIntPositive", "", "6", false},
		{"max=-6", "MaxIntNegative", "", "-6", false},
		{"max=3.0", "MaxFloatPositive", "", "3.0", false},
		{"max=-3.0", "MaxFloatNegative", "", "-3.0", false},

		{"min=3,max=6", "MinMaxIntInt", "3", "6", false},
		{"min=3,max=6.0", "MinMaxIntFloat", "3", "6.0", false},
		{"min=3.0,max=6", "MinMaxFloatInt", "3.0", "6", false},
		{"min=3.0,max=6.0", "MinMaxFloatFloat", "3.0", "6.0", false},

		{"max=6,min=3", "MaxMinIntInt", "3", "6", false},
		{"max=6.0,min=3", "MaxMinIntFloat", "3", "6.0", false},
		{"max=6,min=3.0", "MaxMinFloatInt", "3.0", "6", false},
		{"max=6.0,min=3.0", "MaxMinFloatFloat", "3.0", "6.0", false},

		{"invalid", "Invalid1", "", "", true},
		{"max=", "Invalid2", "", "", true},
		{"min=,max=-3.0", "PartiallyValid1", "", "-3.0", false},
		{"min=5,max=", "PartiallyValid2", "5", "", false},

		{"max=5,max3", "DoubleMax", "", "5", false},
		{"min=5,min3", "DoubleMin", "5", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMin, gotMax, err := stepconf.GetMinMaxValue(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("getMinMaxValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotMin != tt.wantMin {
				t.Errorf("getMinMaxValue() gotMin = %v, want %v", gotMin, tt.wantMin)
			}
			if gotMax != tt.wantMax {
				t.Errorf("getMinMaxValue() gotMax = %v, want %v", gotMax, tt.wantMax)
			}
		})
	}
}
