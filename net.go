package rex

import (
	"bytes"
	"net"
	"net/url"
	"os/exec"
	"strings"
)

func GetFQDN() string {
	cmd := exec.Command("hostname", "-f")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "localhost"
	}
	return string(bytes.TrimSpace(out.Bytes()))
}

func IsTimeout(err error) bool {
	switch err := err.(type) {
	case *url.Error:
		if err, ok := err.Err.(net.Error); ok && err.Timeout() {
			return true
		}
	case net.Error:
		if err.Timeout() {
			return true
		}
	}

	if err != nil && strings.Contains(err.Error(), "use of closed network connection") {
		return true
	}

	return false
}
