package rex

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"strconv"
	"time"

	"github.com/heroku/instruments"
	"github.com/juju/loggo"
	"github.com/julienschmidt/httprouter"
)

type Handler struct {
	http.ResponseWriter
	Request *http.Request
	Params  httprouter.Params
	Log     loggo.Logger
	Start   time.Time
	Timer   *instruments.Timer
}

func NewHandler(w http.ResponseWriter, r *http.Request, params httprouter.Params, timer *instruments.Timer) *Handler {
	return &Handler{
		ResponseWriter: w,
		Request:        r,
		Params:         params,
		Log:            loggo.GetLogger("rex.handler"),
		Start:          time.Now(),
		Timer:          timer,
	}
}

func (h *Handler) Debug(format string, v ...interface{}) {
	h.Header().Add("X-Debug", fmt.Sprintf(format, v...))
}

func (h *Handler) OK(body []byte) {
	h.Debug("OK")
	h.Response(http.StatusOK, body)
}

func (h *Handler) NoContent(format string, v ...interface{}) {
	h.Debug(format, v...)
	h.Response(http.StatusNoContent, nil)
}

func (h *Handler) NotFound(format string, v ...interface{}) {
	h.Debug(format, v...)
	h.Response(http.StatusNotFound, nil)
}

func (h *Handler) BadRequest(format string, v ...interface{}) {
	h.Debug(format, v...)
	h.Response(http.StatusBadRequest, nil)
}

func (h *Handler) ServerError(format string, v ...interface{}) {
	h.Debug(format, v...)
	h.Response(http.StatusInternalServerError, nil)
}

func (h *Handler) Response(status int, body []byte) (int, error) {
	h.Header().Set("Content-Length", strconv.Itoa(len(body)))
	h.Log.Tracef("<<< HTTP/1.1 %d %s", status, h.Header().Get("X-Debug"))
	for key, value := range h.ResponseWriter.Header() {
		h.Log.Tracef("%s: %s", key, value)
	}
	h.WriteHeader(status)

	if h.Timer != nil {
		h.Timer.Update(time.Since(h.Start))
	}

	return h.Write(body)
}

func (h *Handler) LogRequest() {
	LogRequest(h.Log, h.Request)
}

func LogRequest(l loggo.Logger, r *http.Request) {
	if r == nil {
		return
	}
	l.Tracef(">>> %s %s %s from %s", r.Method, r.RequestURI, r.Proto, r.RemoteAddr)
	for key, value := range r.Header {
		l.Tracef("%s: %s", key, value)
	}
}

func LogResponse(l loggo.Logger, r *http.Response) {
	if r == nil {
		return
	}
	l.Tracef("<<< %s %d %s", r.Proto, r.StatusCode, r.Header.Get("X-Debug"))
	for key, value := range r.Header {
		l.Tracef("%s: %s", key, value)
	}
}
