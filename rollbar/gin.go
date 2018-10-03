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

var logger = loggo.GetLogger("gin")

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
				var recoveredError error
				switch err1 := err.(type) {
				case error:
					recoveredError = err1
				default:
					recoveredError = fmt.Errorf("unknown error: %v", err)
				}
				c.Error(recoveredError)
				RequestErrorWithStackSkip(ERR, c.Request, recoveredError, 3)
			}

			if len(c.Errors) == 0 {
				return
			}

			for _, err := range c.Errors {
				LogMeteredError(err,
					"partner="+c.Request.Form.Get("partner"),
					"url="+c.Request.URL.String(),
					"remote_addr="+c.Request.RemoteAddr)
			}

			c.JSON(500, gin.H{
				"errors": c.Errors,
			})
		}()

		c.Next()
	}
}
