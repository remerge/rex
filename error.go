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
