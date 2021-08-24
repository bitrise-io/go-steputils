package stepconf

// InputParser ...
type InputParser interface {
	Parse(input interface{}) error
}

type defaultInputParser struct {
	envGetter EnvGetter
}

// NewInputParser ...
func NewInputParser(envGetter EnvGetter) InputParser {
	return defaultInputParser{
		envGetter: envGetter,
	}
}

// Parse ...
func (p defaultInputParser) Parse(input interface{}) error {
	if err := parse(input, p.envGetter); err != nil {
		return err
	}
	return nil
}
