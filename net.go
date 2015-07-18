package rex

import (
	"bytes"
	"net"
	"net/url"
	"os/exec"
	"strings"
)

func GetFQDN() string {
	out, err := exec.Command("hostname", "-f").Output()
	if err != nil {
		return "localhost"
	}
	return string(bytes.TrimSpace(out))
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
