package kafka

import (
	"fmt"

	"github.com/juju/loggo"
)

type loggerWrapper struct {
	loggo.Logger
}

func (self loggerWrapper) Print(v ...interface{}) {
	self.Infof(fmt.Sprint(v...))
}

func (self loggerWrapper) Println(v ...interface{}) {
	self.Infof(fmt.Sprintln(v...))
}

func (self loggerWrapper) Printf(format string, v ...interface{}) {
	self.Infof(format, v...)
}
