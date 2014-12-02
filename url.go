package rex

import (
	"net/url"
	"strings"

	"golang.org/x/net/publicsuffix"
)

func ParseUrlOrDomain(value string) (*url.URL, error) {
	if !strings.Contains(value, "://") {
		value = "http://" + value
	}
	return url.Parse(value)
}

func UrlTLDPlusOne(value string) (string, error) {
	url, err := ParseUrlOrDomain(value)
	if err != nil {
		return "", err
	}
	return publicsuffix.EffectiveTLDPlusOne(url.Host)
}
