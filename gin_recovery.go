package rex

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http/httputil"
	"runtime"

	"github.com/gin-gonic/gin"
	"github.com/remerge/rex/rollbar"
)

var (
	dunno     = []byte("???")
	centerDot = []byte("·")
	dot       = []byte(".")
	slash     = []byte("/")
)

/**
 * Custom gin recovery middleware that logs to both rollbar and the host
 * service's logger. Most of the functionality is taken from the default Gin
 * recovery middleware
 */
func NewGinRecovery(s *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			recoveredPanicErr := recover()

			// Panic errors are special
			if recoveredPanicErr != nil {
				recoverFromPanickedGin(s, c, recoveredPanicErr)
				return
			}

			// Look for any normal errors and ship them back if possible
			errCount := len(c.Errors)

			if errCount == 0 {
				return
			} else if errCount == 1 {
				logGinError(s, c.Errors[0])

				c.JSON(500, gin.H{
					"error": c.Errors[0].Error(),
				})

				return
			}

			// Piece together multiple errors
			errStrings := make([]string, errCount)

			for i, err := range c.Errors {
				errStrings[i] = err.Error()

				logGinError(s, err)
			}

			c.JSON(500, gin.H{
				"errors": errStrings,
			})
		}()

		c.Next()
	}
}

// Logs an error to both the service log and rollbar
func logGinError(s *Service, err error) {
	errStr := fmt.Sprintf("[gin recovery] %s", err.Error())

	s.Log.Errorf(errStr)
	rollbar.Error(rollbar.ERR, fmt.Errorf(errStr))
}

// Called when the gin recovery handler needs to deal with a panic (should not
// be the usual case)
func recoverFromPanickedGin(s *Service, c *gin.Context, recoveredPanicErr interface{}) {

	// Get a nice stack trace and HTTP req dump
	stack := getStackFrame(3)
	httprequest, _ := httputil.DumpRequest(c.Request, false)

	errStr := fmt.Sprintf(
		"[gin recovery] panic recovered:\n%s\n%s\n%s",
		string(httprequest),
		recoveredPanicErr,
		stack,
	)

	// Ship to rolllbar and anyone who cares
	s.Log.Errorf(errStr)
	rollbar.Error(rollbar.ERR, fmt.Errorf(errStr))

	c.AbortWithStatus(500)
}

// Generates a nicely formated stack frame, skipping skip frames
func getStackFrame(skip int) []byte {
	buff := new(bytes.Buffer)

	// As we loop, we open files and read them. These variables record the
	// currently loaded file.
	var lines [][]byte
	var lastFile string

	// Skip the expected number of frames
	for i := skip; ; i++ {
		pc, file, line, ok := runtime.Caller(i)

		// Couldn't recover anything, bail
		if !ok {
			break
		}

		// Print this much at least. If we can't find the source, it won't show.
		fmt.Fprintf(buff, "%s:%d (0x%x)\n", file, line, pc)

		// Grab line data if the target file has changed
		if file != lastFile {
			data, readErr := ioutil.ReadFile(file)

			if readErr != nil {
				continue
			}

			lines = bytes.Split(data, []byte{'\n'})
			lastFile = file
		}

		fmt.Fprintf(
			buff, "\t%s: %s\n",
			pcFunctionName(pc),
			trimmedSourceLine(lines, line),
		)
	}

	return buff.Bytes()
}

// Attempts to return a space-trimmed version of the specific line in the
// provided source. If the line is not found, a dummy placeholder is generated
func trimmedSourceLine(lines [][]byte, n int) []byte {
	n-- // in stack trace, lines are 1-indexed but our array is 0-indexed

	if n < 0 || n >= len(lines) {
		return dunno
	}

	return bytes.TrimSpace(lines[n])
}

// Returns, if possible, the name of the function containing the PC
func pcFunctionName(pc uintptr) []byte {
	fn := runtime.FuncForPC(pc)

	if fn == nil {
		return dunno
	}

	name := []byte(fn.Name())

	// The name includes the path name to the package, which is unnecessary
	// since the file name is already included.  Plus, it has center dots.
	// That is, we see
	//	runtime/debug.*T·ptrmethod
	// and want
	//	*T.ptrmethod
	// Also the package path might contains dot (e.g. code.google.com/...),
	// so first eliminate the path prefix
	if lastslash := bytes.LastIndex(name, slash); lastslash >= 0 {
		name = name[(lastslash + 1):]
	}

	if period := bytes.Index(name, dot); period >= 0 {
		name = name[(period + 1):]
	}

	name = bytes.Replace(name, centerDot, dot, -1)

	return name
}
