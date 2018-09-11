package rollbar

import (
	"fmt"
	"sort"
	"strings"

	"bitbucket.org/pkg/inflect"
	"github.com/gin-gonic/gin"
	"github.com/juju/loggo"
	"github.com/rcrowley/go-metrics"
	"github.com/remerge/go-lock_free_timer"
)

var (
	logger         = loggo.GetLogger("gin")
	ginErrPrefixes = []string{
		"gin.Error: invalid URL escape",
		"gin.Error: read tcp",
	}
)

// LogMeteredError writes message to log as well as increases "track_error"
// metric. First 32 runes of error are used for "reason" label in
// URL-parametrised form. All extra params which begins from "m+" prefix will
// be used as labels.
func LogMeteredError(err error, extra ...string) {
	// try to strip reason as possible
	split := strings.SplitN(err.Error(), ":", 2)
	labels := []string{fmt.Sprintf("reason=%.32s", inflect.Parameterize(split[0]))}

	line := []string{fmt.Sprintf("%v:", err)}
	sort.Strings(extra)
	for _, v := range extra {
		if strings.HasSuffix(v, "=") {
			v += "none"
		}
		if strings.HasPrefix(v, "m+") {
			v = strings.TrimPrefix(v, "m+")
			labels = append(labels, strings.TrimPrefix(v, "m+"))
			continue
		}
		line = append(line, v)
	}
	metrics.GetOrRegisterCounter("track,"+strings.Join(labels, ",")+" error", lft.DefaultRegistry).Inc(1)
	logger.LogCallf(3, loggo.ERROR, strings.Join(append(line, labels...), " "))
}

func GinRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				switch err.(type) {
				case error:
					c.Error(err.(error))
				default:
					c.Error(fmt.Errorf("unknown error: %v", err))
				}
			}

			if len(c.Errors) == 0 {
				return
			}

		errReportLoop:
			for _, err := range c.Errors {
				LogMeteredError(err,
					"m+partner="+c.Request.Form.Get("partner"),
					"url="+c.Request.URL.String(),
					"remote_addr="+c.Request.RemoteAddr)
				for _, prefix := range ginErrPrefixes {
					if strings.HasPrefix(err.Error(), prefix) {
						continue errReportLoop
					}
				}
				RequestErrorWithStackSkip(ERR, c.Request, err, 3)
			}

			c.JSON(500, gin.H{
				"errors": c.Errors,
			})
		}()

		c.Next()
	}
}
