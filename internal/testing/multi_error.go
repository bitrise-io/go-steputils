package testing

// MultiError aggregates multiple errors into one.
type MultiError []error

func (m MultiError) Error() string {
	if len(m) == 0 {
		return ""
	}
	// join messages; you can customize formatting
	b := make([]byte, 0, 128)
	for i, err := range m {
		if err == nil {
			continue
		}
		if i > 0 {
			b = append(b, '\n')
		}
		b = append(b, err.Error()...)
	}
	return string(b)
}

// AppendErr appends err to MultiError if err is not nil.
func AppendErr(m *MultiError, err error) {
	if err == nil {
		return
	}
	*m = append(*m, err)
}
