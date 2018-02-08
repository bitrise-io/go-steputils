package stepconfig

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/bitrise-io/go-utils/log"
)

// ErrInvalidConfig indicates that a configuration is of the wrong type.
var ErrInvalidConfig = errors.New("conf must be a struct pointer")

// ParseError occurs when a struct field cannot be set.
type ParseError struct {
	Field string
	Value string
	Err   error
}

// Error implements builtin errors.Error
func (e *ParseError) Error() string {
	segments := []string{e.Field}
	if e.Value != "" {
		segments = append(segments, e.Value)
	}
	segments = append(segments, e.Err.Error())
	return strings.Join(segments, ": ")
}

// Secret variables are not shown in the printed output
type Secret string

// String implements fmt.Stringer.String.
// When a Secret is printed, it's masking the underlying string with asterisks.
func (s Secret) String() string {
	return strings.Repeat("*", 5)
}

// Print the name of the struct in blue color followed by a newline,
// then print all fields and their respective values in separate lines.
func Print(config interface{}) {
	v := reflect.ValueOf(config)
	t := reflect.TypeOf(config)

	log.Infof("%s:\n", t.Name())
	for i := 0; i < t.NumField(); i++ {
		fmt.Printf("- %s: %v\n", t.Field(i).Name, v.Field(i).Interface())
	}
}

// parseTag splits a struct field's env tag into its name and option.
func parseTag(tag string) (string, string) {
	if idx := strings.Index(tag, ","); idx != -1 {
		return tag[:idx], tag[idx+1:]
	}
	return tag, ""
}

// Parse populates a struct with the retrieved values from environment variables
// described by struct tags.
func Parse(conf interface{}) error {
	c := reflect.ValueOf(conf)
	if c.Kind() != reflect.Ptr {
		return ErrInvalidConfig
	}
	c = c.Elem()
	if c.Kind() != reflect.Struct {
		return ErrInvalidConfig
	}
	t := c.Type()

	var errs []*ParseError
	for i := 0; i < c.NumField(); i++ {
		tag, ok := t.Field(i).Tag.Lookup("env")
		if !ok {
			continue
		}
		key, constraint := parseTag(tag)
		value := os.Getenv(key)

		if err := setField(c.Field(i), value, constraint); err != nil {
			errs = append(errs, &ParseError{t.Field(i).Name, value, err})
		}
	}
	if len(errs) > 0 {
		errorString := "failed to parse config:"
		for _, err := range errs {
			errorString += fmt.Sprintf("\n- %s", err)
		}
		return errors.New(errorString)
	}

	return nil
}

func setField(f reflect.Value, value, constraint string) error {
	switch constraint {
	case "":
		break
	case "required":
		if value == "" {
			return errors.New("required variable is not present")
		}
	case "file", "dir":
		if err := checkPath(value, constraint == "dir"); err != nil {
			return err
		}
	// TODO: use FindStringSubmatch to distinguish no match and match for empty string.
	case regexp.MustCompile(`^opt\[.*\]$`).FindString(constraint):
		if !contains(value, constraint) {
			// TODO: print only the value options, not the whole string.
			return fmt.Errorf("value is not in value options (%s)", constraint)
		}
	default:
		return fmt.Errorf("invalid constraint (%s)", constraint)
	}

	if value != "" {
		return setValue(f, value)
	}
	return nil
}

func setValue(field reflect.Value, value string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Bool:
		field.SetBool(value == "yes" || value == "Yes")
	case reflect.Int:
		n, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return errors.New("can't convert to int")
		}
		field.SetInt(n)
	case reflect.Slice:
		field.Set(reflect.ValueOf(strings.Split(value, "|")))
	default:
		return fmt.Errorf("type %q is not supported", field.Kind())
	}
	return nil
}

func checkPath(path string, dir bool) error {
	file, err := os.Stat(path)
	if err != nil {
		// TODO: check case when file exist but os.Stat fails.
		return os.ErrNotExist
	}
	if dir && !file.IsDir() {
		return errors.New("not a directory")
	}
	return nil
}

func contains(s, opts string) bool {
	// TODO: improve readability.
	for _, opt := range strings.Split(opts[strings.Index(opts, "[")+1:len(opts)-1], ",") {
		if opt == s {
			return true
		}
	}
	return false
}
