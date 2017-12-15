package input_test

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/kdobmayer/input"
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
	Name         string       `env:"name"`
	BuildNumber  int          `env:"build_number"`
	IsUpdate     bool         `env:"is_update"`
	Items        []string     `env:"items"`
	Password     input.Secret `env:"password"`
	Empty        string       `env:"empty"`
	Mandatory    string       `env:"mandatory" validate:"required"`
	TempFile     string       `env:"file" validate:"path"`
	TempDir      string       `env:"dir" validate:"dir"`
	ExportMethod string       `env:"export_method" opts:"dev,qa,prod"`
}

func TestNew(t *testing.T) {
	var c Config
	os.Clearenv()
	setEnvironment(valid)

	err := input.New(&c)
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
	if err := input.New(c); err == nil {
		t.Error("no failure when input parameter is a pointer")
	}
}

func TestNotStruct(t *testing.T) {
	var basicType string
	if err := input.New(&basicType); err == nil {
		t.Error("no failure when input parameter is not a struct")
	}
}

func TestInvalidEnvs(t *testing.T) {
	setEnvironment(invalid)
	var c Config
	if err := input.New(&c); err == nil {
		t.Error("no failure when invalid values used")
	}
}

func TestValidateNotExists(t *testing.T) {
	type invalid struct {
		Length string `env:"length" validate:"length"`
	}
	var c invalid
	if err := input.New(&c); err == nil {
		t.Error("no failure when validate tag is not exists")
	}
}

func TestRequired(t *testing.T) {
	type config struct {
		Required string `env:"required" validate:"required"`
	}
	var c config
	os.Clearenv()

	if err := input.New(&c); err == nil {
		t.Error("no failure when required env var is missing")
	}

	err := os.Setenv("required", "set")
	if err != nil {
		t.Fatalf("should not have error: %s", err)
	}
	if err := input.New(&c); err != nil {
		t.Error("failure when required env var is set")
	}
}

func TestValidatePath(t *testing.T) {
	type config struct {
		Path string `env:"path" validate:"path"`
	}
	var c config
	os.Clearenv()

	if err := os.Setenv("path", "/not/exist"); err != nil {
		t.Fatalf("should not have error: %s", err)
	}
	if err := input.New(&c); err == nil {
		t.Error("no failure when path does not exist")
	}

	f, err := ioutil.TempFile("", "input_test")
	if err != nil {
		t.Fatalf("should not have error: %s", err)
	}
	if err := os.Setenv("path", f.Name()); err != nil {
		t.Fatalf("should not have error: %s", err)
	}
	if err := input.New(&c); err != nil {
		t.Error("failure when path is exist")
	}
}

func TestValidateDir(t *testing.T) {
	type config struct {
		Dir string `env:"dir" validate:"dir"`
	}
	var c config
	os.Clearenv()

	if err := os.Setenv("dir", "/not/exist"); err != nil {
		t.Fatalf("should not have error: %s", err)
	}
	if err := input.New(&c); err == nil {
		t.Error("no failure when dir does not exist")
	}

	dir, err := ioutil.TempDir("", "input_test")
	if err != nil {
		t.Fatalf("should not have error: %s", err)
	}
	if err := os.Setenv("dir", dir); err != nil {
		t.Fatalf("should not have error: %s", err)
	}
	if err := input.New(&c); err != nil {
		t.Error("failure when dir does exist")
	}
}

func TestValueOptions(t *testing.T) {
	type config struct {
		Option string `env:"option" opts:"opt1,opt2,opt3"`
	}
	var c config
	os.Clearenv()

	if err := os.Setenv("option", "no-opt"); err != nil {
		t.Fatalf("should not have error: %s", err)
	}
	if err := input.New(&c); err == nil {
		t.Error("no failure when value is not in value options")
	}

	if err := os.Setenv("option", "opt1"); err != nil {
		t.Fatalf("should not have error: %s", err)
	}
	if err := input.New(&c); err != nil {
		t.Error("failure when value is in value options")
	}
}

func ExampleNew() {
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
	if err := input.New(&c); err != nil {
		log.Fatal(err)
	}
	fmt.Println(c)
	// Output: {example 1548}
}
