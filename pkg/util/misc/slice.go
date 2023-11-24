package misc

import "strings"

func InSliceString(needle string, stack []string) bool {
	if len(stack) == 0 {
		return false
	}

	for _, item := range stack {
		if needle == item {
			return true
		}
	}
	return false
}

func InSliceInt(needle int, stack []int) bool {
	if len(stack) == 0 {
		return false
	}

	for _, item := range stack {
		if needle == item {
			return true
		}
	}
	return false
}

// InSlicePrefixString returns true, if needle has prefix of any of the prefixes string slice
func HasPrefixInSlice(needle string, prefixes []string) bool {
	if len(prefixes) == 0 {
		return false
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(needle, prefix) {
			return true
		}
	}
	return false
}
