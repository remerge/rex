package rex

import "regexp"

var uuidRegex = regexp.MustCompile("^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-4[a-fA-F0-9]{3}-[8|9|aA|bB][a-fA-F0-9]{3}-[a-fA-F0-9]{12}$")

/**
 * Checks the format of the provided UUID string for valid length and
 * delimiting dashes. Only a structural check!
 */
func FastValidateUUID(uuid string) bool {
	return len(uuid) == 36 && uuid[8] == '-' && uuid[13] == '-' && uuid[18] == '-'
}

/**
 * A more comprehensive regex UUID check
 */
func ValidateUUID(uuid string) bool {
	return uuidRegex.MatchString(uuid)
}
