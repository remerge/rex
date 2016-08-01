package rollbar

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"runtime"
	"strings"
)

var (
	dunno     = []byte("???")
	centerDot = []byte("·")
	dot       = []byte(".")
	slash     = []byte("/")
)

var (
	knownFilePathPatterns []string = []string{
		"github.com/",
		"code.google.com/",
		"bitbucket.org/",
		"launchpad.net/",
	}
)

type Frame struct {
	Filename   string `json:"filename"`
	Method     string `json:"method"`
	SourceLine string `json:"-"`
	Line       int    `json:"lineno"`
}

func (frame Frame) String() string {
	return fmt.Sprintf("  %s:%d\n\t%s: %s", frame.Filename, frame.Line, frame.Method, frame.SourceLine)
}

type Stack []Frame

func BuildStack(skip int) Stack {
	var lines [][]byte
	var lastFile string

	stack := make(Stack, 0)

	for i := skip; ; i++ {
		pc, file, line, ok := runtime.Caller(i)

		if !ok {
			break
		}

		// Grab line data if the target file has changed
		if file != lastFile {
			data, readErr := ioutil.ReadFile(file)

			if readErr != nil {
				continue
			}

			lines = bytes.Split(data, []byte{'\n'})
			lastFile = file
		}

		stack = append(stack, Frame{
			shortenFilePath(file),
			string(functionName(pc)),
			string(trimmedSourceLine(lines, line)),
			line,
		})
	}

	return stack
}

func (stack Stack) String() (s string) {
	frames := make([]string, 0)

	for _, frame := range stack {
		frames = append(frames, frame.String())
	}

	return strings.Join(frames, "\n")
}

// Remove un-needed information from the source file path. This makes them
// shorter in Rollbar UI as well as making them the same, regardless of the
// machine the code was compiled on.
//
// Examples:
//   /usr/local/go/src/pkg/runtime/proc.c -> pkg/runtime/proc.c
//   /home/foo/go/src/github.com/rollbar/rollbar.go -> github.com/rollbar/rollbar.go
func shortenFilePath(s string) string {
	idx := strings.Index(s, "/src/")
	if idx != -1 {
		return s[idx+5:]
	}

	for _, pattern := range knownFilePathPatterns {
		idx = strings.Index(s, pattern)
		if idx != -1 {
			return s[idx:]
		}
	}

	return s
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

func functionName(pc uintptr) []byte {
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
