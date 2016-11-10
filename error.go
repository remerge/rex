package rex

import (
	"fmt"

	"github.com/remerge/rex/rollbar"
)

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
// propagate (always)
func MergeErrors(a error, b error) error {
	return fmt.Errorf("%s\n%s", a.Error(), b.Error())
}
