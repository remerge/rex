package rex

import (
	"errors"
	"fmt"

	"github.com/getsentry/raven-go"
	"github.com/juju/loggo"
)

// so we don't have to pass the raven client through every function
var Raven *raven.Client

func MayPanic(err error) {
	if err == nil {
		return
	}

	ravenErr := <-CaptureError(err)
	if ravenErr != nil {
		loggo.GetLogger("rex.error").Errorf("failed to send panic to sentry: %s", ravenErr)
	}

	panic(err)
}

func MayPanicNew(format string, v ...interface{}) {
	MayPanic(errors.New(fmt.Sprintf(format, v...)))
}

func CaptureError(err error) chan error {
	if err == nil {
		return nil
	}

	loggo.GetLogger("rex.error").Errorf(err.Error())

	packet := raven.NewPacket(err.Error(), raven.NewException(err, raven.NewStacktrace(1, 3, nil)))
	_, ch := Raven.Capture(packet, nil)
	return ch
}

func CaptureErrorNew(format string, v ...interface{}) chan error {
	return CaptureError(errors.New(fmt.Sprintf(format, v...)))
}

func WithRecover(fn func() error) error {
	var panicValue interface{}
	result := func() error {
		defer func() { panicValue = recover() }()
		return fn()
	}()

	if result == nil && panicValue == nil {
		return nil
	}

	if panicValue != nil {
		switch panicValue.(type) {
		case error:
			return panicValue.(error)
		default:
			return errors.New(fmt.Sprintf("caught non-error panic: %#v", panicValue))
		}
	}

	return result
}
