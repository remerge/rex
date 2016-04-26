package rollbar

import "github.com/juju/loggo"

func init() {
	loggo.ConfigureLoggers("rollbar.message=CRITICAL")
}
