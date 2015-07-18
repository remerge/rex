package rex

import (
	"fmt"
	"runtime/debug"

	"github.com/juju/loggo"
	"github.com/stvp/rollbar"
)

func MayPanic(err error) {
	if err != nil {
		CaptureError(err)
		panic(err)
	}
}

func MayPanicNew(format string, v ...interface{}) {
	MayPanic(fmt.Errorf(format, v...))
}

func CaptureError(err error) {
	if err != nil {
		loggo.GetLogger("rex.error").Errorf(err.Error())
		debug.PrintStack()
		rollbar.Error(rollbar.ERR, err)
	}
}

func CaptureErrorNew(format string, v ...interface{}) {
	CaptureError(fmt.Errorf(format, v...))
}
