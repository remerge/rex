package rollbar

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/adler32"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/remerge/rex/log"
)

const (
	NAME    = "go-rollbar"
	VERSION = "0.3.1"

	// Severity levels
	CRIT  = "critical"
	ERR   = "error"
	WARN  = "warning"
	INFO  = "info"
	DEBUG = "debug"

	FILTERED = "[FILTERED]"
)

// Config holds the Rollbar client settings
type Config struct {
	Token        string
	Hostname     string
	Environment  string
	Platform     string
	Endpoint     string
	Buffer       int
	FilterFields *regexp.Regexp
	ErrorWriter  io.Writer
	CodeVersion  string
}

func (c *Config) Merge(other *Config) *Config {
	merged := *c

	if other.Token != "" {
		merged.Token = other.Token
	}

	if other.Hostname != "" {
		merged.Hostname = other.Hostname
	}

	if other.Environment != "" {
		merged.Environment = other.Environment
	}

	if other.Platform != "" {
		merged.Platform = other.Platform
	}

	if other.Buffer != 0 {
		merged.Buffer = other.Buffer
	}

	if other.FilterFields != nil {
		merged.FilterFields = other.FilterFields
	}

	if other.ErrorWriter != nil {
		merged.ErrorWriter = other.ErrorWriter
	}

	if other.CodeVersion != "" {
		merged.CodeVersion = other.CodeVersion
	}

	return &merged
}

// Global Rollbar configuration, also used as defaults
var GlobalConfig = &Config{
	// Rollbar access token. If this is blank, no errors will be reported to
	// Rollbar.
	Token: "",

	// Hostname that will be submitted for each message
	Hostname: "localhost",

	// All errors and messages will be submitted under this environment.
	Environment: "development",

	// Platform, default to OS, but could be change ('client' for instance)
	Platform: runtime.GOOS,

	// API endpoint for Rollbar.
	Endpoint: "https://api.rollbar.com/api/1/item/",

	// Maximum number of errors allowed in the sending queue before we start
	// dropping new errors on the floor.
	Buffer: 1000,

	// Filter GET and POST parameters from being sent to Rollbar.
	FilterFields: regexp.MustCompile("password|secret|token"),

	// Output of error, by default stderr
	ErrorWriter: os.Stderr,

	// All errors and messages will be submitted under this code
	// version. If this is blank no value will be sent
	CodeVersion: "",
}

// Client is responsible for sending data to Rollbar
type Client struct {
	config *Config

	// Queue of messages to be sent.
	bodyChannel chan map[string]interface{}
	waitGroup   sync.WaitGroup
}

// Global Rollbar client
var GlobalClient = NewClient(GlobalConfig)

// Dispatch to global client
func Error(level string, err error, fields ...*Field) error {
	return GlobalClient.Error(level, err, fields...)
}

func ErrorWithStackSkip(level string, err error, skip int, fields ...*Field) error {
	return GlobalClient.ErrorWithStackSkip(level, err, skip, fields...)
}

func ErrorWithStack(level string, err error, stack Stack, fields ...*Field) error {
	return GlobalClient.ErrorWithStack(level, err, stack, fields...)
}

func RequestError(level string, r *http.Request, err error, fields ...*Field) error {
	return GlobalClient.RequestError(level, r, err, fields...)
}

func RequestErrorWithStackSkip(level string, r *http.Request, err error, skip int, fields ...*Field) error {
	return GlobalClient.RequestErrorWithStackSkip(level, r, err, skip, fields...)
}

func RequestErrorWithStack(level string, r *http.Request, err error, stack Stack, fields ...*Field) error {
	return GlobalClient.RequestErrorWithStack(level, r, err, stack, fields...)
}

func Message(level string, msg string, fields ...*Field) {
	GlobalClient.Message(level, msg, fields...)
}

func Wait() {
	GlobalClient.Wait()
}

// Fields can be used to pass arbitrary data to the Rollbar API.
type Field struct {
	Name string
	Data interface{}
}

// -- Constructor

func NewClient(config *Config) *Client {
	out, err := exec.Command("hostname", "-f").Output()
	if err != nil {
		config.Hostname = "localhost"
	} else {
		config.Hostname = string(bytes.TrimSpace(out))
	}

	client := &Client{
		config:      config,
		bodyChannel: make(chan map[string]interface{}, config.Buffer),
	}

	go func() {
		for body := range client.bodyChannel {
			client.post(body)
			client.waitGroup.Done()
		}
	}()

	return client
}

func NewClientFromGlobal(config *Config) *Client {
	return NewClient(GlobalConfig.Merge(config))
}

// -- Error reporting

// Error asynchronously sends an error to Rollbar with the given severity
// level. You can pass, optionally, custom Fields to be passed on to Rollbar.
func (c *Client) Error(level string, err error, fields ...*Field) error {
	if err != nil {
		c.ErrorWithStackSkip(level, err, 1, fields...)
	}
	return err
}

// ErrorWithStackSkip asynchronously sends an error to Rollbar with the given
// severity level and a given number of stack trace frames skipped. You can
// pass, optionally, custom Fields to be passed on to Rollbar.
func (c *Client) ErrorWithStackSkip(level string, err error, skip int, fields ...*Field) error {
	stack := BuildStack(2 + skip)
	c.ErrorWithStack(level, err, stack, fields...)
	return err
}

// ErrorWithStack asynchronously sends and error to Rollbar with the given
// stacktrace and (optionally) custom Fields to be passed on to Rollbar.
func (c *Client) ErrorWithStack(level string, err error, stack Stack, fields ...*Field) error {
	log.GetLogger("rollbar").Errorf("%v", err)
	fmt.Printf("\n%s\n\n", stack.String())
	c.buildAndPushError(level, err, stack, fields...)
	return err
}

// RequestError asynchronously sends an error to Rollbar with the given
// severity level and request-specific information. You can pass, optionally,
// custom Fields to be passed on to Rollbar.
func (c *Client) RequestError(level string, r *http.Request, err error, fields ...*Field) error {
	if err != nil {
		c.RequestErrorWithStackSkip(level, r, err, 1, fields...)
	}
	return err
}

// RequestErrorWithStackSkip asynchronously sends an error to Rollbar with the
// given severity level and a given number of stack trace frames skipped, in
// addition to extra request-specific information. You can pass, optionally,
// custom Fields to be passed on to Rollbar.
func (c *Client) RequestErrorWithStackSkip(level string, r *http.Request, err error, skip int, fields ...*Field) error {
	stack := BuildStack(2 + skip)
	c.RequestErrorWithStack(level, r, err, stack, fields...)
	return err
}

// RequestErrorWithStack asynchronously sends an error to Rollbar with the
// given severity level, request-specific information provided by the given
// http.Request, and a custom Stack. You You can pass, optionally, custom
// Fields to be passed on to Rollbar.
func (c *Client) RequestErrorWithStack(level string, r *http.Request, err error, stack Stack, fields ...*Field) error {
	log.GetLogger("rollbar").Errorf("%v", err)
	requestDump, _ := httputil.DumpRequest(r, false)
	fmt.Printf("\n%s%s\n\n", requestDump, stack.String())
	c.buildAndPushError(level, err, stack, append(fields, &Field{Name: "request", Data: c.errorRequest(r)})...)
	return err
}

func (c *Client) buildError(level string, err error, stack Stack, fields ...*Field) map[string]interface{} {
	body := c.buildBody(level, fmt.Sprintf("%v", err))
	data := body["data"].(map[string]interface{})
	errBody, fingerprint := c.errorBody(err, stack)
	data["body"] = errBody
	data["fingerprint"] = fingerprint

	for _, field := range fields {
		data[field.Name] = field.Data
	}

	return body
}

func (c *Client) buildAndPushError(level string, err error, stack Stack, fields ...*Field) {
	c.push(c.buildError(level, err, stack, fields...))
}

// -- Message reporting

// Message asynchronously sends a message to Rollbar with the given severity
// level.
func (c *Client) Message(level string, msg string, fields ...*Field) {
	body := c.buildBody(level, msg)
	data := body["data"].(map[string]interface{})
	data["body"] = c.messageBody(msg)

	for _, field := range fields {
		data[field.Name] = field.Data
	}

	c.push(body)
}

// -- Misc.

// Wait will block until the queue of errors / messages is empty. This allows
// you to ensure that errors / messages are sent to Rollbar before exiting an
// application.
func (c *Client) Wait() {
	c.waitGroup.Wait()
}

// Build the main JSON structure that will be sent to Rollbar with the
// appropriate metadata.
func (c *Client) buildBody(level, title string) map[string]interface{} {
	timestamp := time.Now().Unix()

	data := map[string]interface{}{
		"environment": c.config.Environment,
		"title":       title,
		"level":       level,
		"timestamp":   timestamp,
		"platform":    c.config.Platform,
		"language":    "go",
		"server": map[string]interface{}{
			"host": c.config.Hostname,
		},
		"notifier": map[string]interface{}{
			"name":    NAME,
			"version": VERSION,
		},
	}
	if c.config.CodeVersion != "" {
		data["code_version"] = c.config.CodeVersion
	}

	return map[string]interface{}{
		"access_token": c.config.Token,
		"data":         data,
	}
}

// errorBody generates a Rollbar error body with a given stack trace.
func (c *Client) errorBody(err error, stack Stack) (map[string]interface{}, string) {
	fingerprint := stack.Fingerprint()
	errBody := map[string]interface{}{
		"trace": map[string]interface{}{
			"frames": stack,
			"exception": map[string]interface{}{
				"class":   c.errorClass(err),
				"message": fmt.Sprintf("%v", err),
			},
		},
	}
	return errBody, fingerprint
}

// errorRequest extracts details from a Request in a format that Rollbar
// accepts.
func (c *Client) errorRequest(r *http.Request) map[string]interface{} {
	cleanQuery := c.filterParams(r.URL.Query())

	return map[string]interface{}{
		"url":     r.URL.String(),
		"method":  r.Method,
		"headers": c.flattenValues(r.Header),

		// GET params
		"query_string": url.Values(cleanQuery).Encode(),
		"GET":          c.flattenValues(cleanQuery),

		// POST / PUT params
		"POST": c.flattenValues(c.filterParams(r.Form)),
	}
}

// filterParams filters sensitive information like passwords from being sent to
// Rollbar.
func (c *Client) filterParams(values map[string][]string) map[string][]string {
	for key := range values {
		if c.config.FilterFields.Match([]byte(key)) {
			values[key] = []string{FILTERED}
		}
	}

	return values
}

func (c *Client) flattenValues(values map[string][]string) map[string]interface{} {
	result := make(map[string]interface{})

	for k, v := range values {
		if len(v) == 1 {
			result[k] = v[0]
		} else {
			result[k] = v
		}
	}

	return result
}

// Build a message inner-body for the given message string.
func (c *Client) messageBody(s string) map[string]interface{} {
	return map[string]interface{}{
		"message": map[string]interface{}{
			"body": s,
		},
	}
}

func (c *Client) errorClass(err error) string {
	class := reflect.TypeOf(err).String()
	if class == "" {
		return "panic"
	} else if class == "*errors.errorString" {
		checksum := adler32.Checksum([]byte(fmt.Sprint(err)))
		return fmt.Sprintf("{%x}", checksum)
	} else {
		return strings.TrimPrefix(class, "*")
	}
}

// -- POST handling

// Queue the given JSON body to be POSTed to Rollbar.
func (c *Client) push(body map[string]interface{}) {
	if len(c.bodyChannel) < c.config.Buffer {
		c.waitGroup.Add(1)
		c.bodyChannel <- body
	}
}

// POST the given JSON body to Rollbar synchronously.
func (c *Client) post(body map[string]interface{}) {
	//if Environment == "development" || Environment == "test" {
	//    return
	//}

	if len(c.config.Token) == 0 {
		c.stderr("empty token")
		return
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		c.stderr("failed to encode payload: %v", err)
		return
	}

	resp, err := http.Post(c.config.Endpoint, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		c.stderr("POST failed: %v", err)
	} else if resp.StatusCode != 200 {
		c.stderr("received response: %s", resp.Status)
	}
	if resp != nil {
		_ = resp.Body.Close()
	}
}

// -- stderr
func (c *Client) stderr(format string, args ...interface{}) {
	if c.config.ErrorWriter != nil {
		format = "Rollbar error: " + format + "\n"
		fmt.Fprintf(c.config.ErrorWriter, format, args...)
	}
}
