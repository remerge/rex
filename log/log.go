package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/juju/ansiterm"
	"github.com/juju/loggo"
	"github.com/remerge/rex/env"
)

var logContext *loggo.Context

func GetLogger(name string) loggo.Logger {
	return logContext.GetLogger(name)
}

func LoggerInfo() string {
	return logContext.Config().String()
}

func ConfigureLoggers(specification string) error {
	config, err := loggo.ParseConfigString(specification)
	if err != nil {
		return err
	}
	logContext.ApplyConfig(config)
	return nil
}

func init() {
	logContext = loggo.NewContext(loggo.INFO)
	logContext.AddWriter("default", newWriter(os.Stdout, os.Stderr))
}

type loggoWriter struct {
	out *ansiterm.Writer
	err *ansiterm.Writer
}

func newWriter(out, err io.Writer) loggo.Writer {
	return &loggoWriter{
		out: ansiterm.NewWriter(out),
		err: ansiterm.NewWriter(err),
	}
}

func (w *loggoWriter) Write(entry loggo.Entry) {
	writer := w.out

	if entry.Level >= loggo.ERROR {
		writer = w.err
	}

	if !env.IsProd() {
		ts := entry.Timestamp.In(time.UTC).Format("2006-01-02 15:04:05")
		fmt.Fprintf(writer, "%s ", ts)
	}

	filename := filepath.Base(entry.Filename)

	fmt.Fprintf(
		writer,
		"%s [%s] %s (in %s:%d)\n",
		entry.Level.Short(),
		entry.Module,
		entry.Message,
		filename,
		entry.Line,
	)
}
