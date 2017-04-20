package rex

import (
	"fmt"

	"github.com/remerge/rex/rollbar"
)

// Returns a multi-line error containing all of the supplied errors, with nil
// values excluded
func CompositeError(errors ...error) error {
	finalErrorString := ""

	for i := 0; i < len(errors); i++ {
		if errors[i] != nil {
			if len(finalErrorString) > 0 {
				finalErrorString += "\n"
			}

			finalErrorString += errors[i].Error()
		}
	}

	return fmt.Errorf(finalErrorString)
}

func MayPanic(err error) {
	if err != nil {
		rollbar.Error(rollbar.CRIT, err)
		rollbar.Wait()
		panic(err)
	}
}

func MayPanicNew(format string, v ...interface{}) {
	MayPanic(fmt.Errorf(format, v...))
}

// Creates a new error with both messages seperated by a newline
// Useful in defer blocks at the end of methods, where errors should still
// propagate (always). Either error may be nil
func MergeErrors(a error, b error) error {
	if a == nil {
		return b
	} else if b == nil {
		return a
	}

	return fmt.Errorf("%s\n%s", a.Error(), b.Error())
}
