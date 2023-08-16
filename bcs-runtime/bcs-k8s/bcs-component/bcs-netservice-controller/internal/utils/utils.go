package utils

// StringInSlice split string to slice
func StringInSlice(strs []string, str string) bool {
	for _, item := range strs {
		if str == item {
			return true
		}
	}
	return false
}

// RemoveStringInSlice remove string from slice
func RemoveStringInSlice(strs []string, str string) []string {
	var newSlice []string
	for _, s := range strs {
		if s != str {
			newSlice = append(newSlice, s)
		}
	}
	return newSlice
}
