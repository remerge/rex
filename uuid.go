package rex

import "regexp"

var uuidRegex = regexp.MustCompile("^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-4[a-fA-F0-9]{3}-[8|9|aA|bB][a-fA-F0-9]{3}-[a-fA-F0-9]{12}$")
var uuidRegexiOS = regexp.MustCompile("^[A-F0-9]{8}-[A-F0-9]{4}-4[A-F0-9]{3}-[8|9|A|B][A-F0-9]{3}-[A-F0-9]{12}$")
var uuidRegexAndroid = regexp.MustCompile("^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[8|9|a|b][a-f0-9]{3}-[a-f0-9]{12}$")

/**
 * Checks the format of the provided UUID string for valid length and
 * delimiting dashes. Only a structural check!
 */
func FastValidateUUID(uuid string) bool {
	return len(uuid) == 36 && uuid[8] == '-' && uuid[13] == '-' && uuid[18] == '-'
}

// ValidateUUID return true iff uuid matches the UUID standard regexp
func ValidateUUID(uuid string) bool {
	return uuidRegex.MatchString(uuid)
}

// IsiOSUUID returns true if uuid matches the iOS specifc UUID regexp
func IsiOSUUID(uuid string) bool {
	return uuidRegexiOS.MatchString(uuid)
}

// IsAndroidUUID returns true if uuid matches the Android specifc UUID regexp
func IsAndroidUUID(uuid string) bool {
	return uuidRegexAndroid.MatchString(uuid)
}
