package tools

// IsValueInStringSlice ...
func IsValueInStringSlice(value string, slice []string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}
