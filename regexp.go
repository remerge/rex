package rex

import "regexp"

func MatchString(pattern string, s string) bool {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(s)
}

func Match(pattern string, b []byte) bool {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	return re.Match(b)
}
