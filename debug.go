package rex

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/pprof"
	"runtime"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/juju/loggo"
	"github.com/remerge/rex/rollbar"
)

func Inspect(v interface{}) string {
	bytes, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%#v", v)
	}
	return string(bytes)
}

func StartDebugServer(port int) *gin.Engine {
	log := loggo.GetLogger("rex.debug")
	r := gin.Default()

	r.GET("/loggo", getLoggoSpec)
	r.POST("/loggo", setLoggoSpec)

	r.GET("/debug/pprof/", gin.WrapF(pprof.Index))
	r.GET("/debug/pprof/block", gin.WrapF(pprof.Index))
	r.GET("/debug/pprof/cmdline", gin.WrapF(pprof.Cmdline))
	r.GET("/debug/pprof/goroutine", gin.WrapF(pprof.Index))
	r.GET("/debug/pprof/heap", gin.WrapF(pprof.Index))
	r.GET("/debug/pprof/profile", gin.WrapF(pprof.Profile))
	r.GET("/debug/pprof/symbol", gin.WrapF(pprof.Symbol))
	r.GET("/debug/pprof/threadcreate", gin.WrapF(pprof.Index))
	r.GET("/debug/pprof/trace", gin.WrapF(pprof.Trace))

	r.GET("/blockprof/:rate", func(c *gin.Context) {
		r, err := strconv.Atoi(c.Param("rate"))
		if err != nil {
			c.String(http.StatusOK, "rate invalid %s. %v", c.Param("rate"), err)
			return
		}
		runtime.SetBlockProfileRate(r)
		c.String(http.StatusOK, "new rate %d", r)
	})

	log.Infof("starting debug server on port=%d", port)
	go func() {
		MayPanic(r.Run(fmt.Sprintf(":%d", port)))
	}()

	return r
}

func getLoggoSpec(c *gin.Context) {
	c.String(200, loggo.LoggerInfo())
}

func setLoggoSpec(c *gin.Context) {
	log := loggo.GetLogger("rex.debug")

	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		rollbar.Error(rollbar.WARN, err)
		c.AbortWithStatus(500)
		return
	}

	log.Infof("setting new loggo spec: %s", string(body))
	err = loggo.ConfigureLoggers(string(body))
	if err != nil {
		rollbar.Error(rollbar.WARN, err)
		c.AbortWithStatus(400)
		return
	}

	c.String(200, loggo.LoggerInfo())
}
