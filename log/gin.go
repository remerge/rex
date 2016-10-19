package log

import (
	"io/ioutil"
	"time"

	"github.com/gin-gonic/gin"
)

func GinLogger(name string) gin.HandlerFunc {
	log := GetLogger(name)
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Debugf("%s %s -> %d in %v %s",
			c.Request.Method,
			c.Request.URL.Path,
			c.Writer.Status(),
			time.Now().Sub(start),
			c.Errors.String(),
		)
	}
}

func GetLoggoSpec(c *gin.Context) {
	c.String(200, LoggerInfo())
}

func SetLoggoSpec(c *gin.Context) {
	log := GetLogger("rex.debug")

	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatus(500)
		return
	}

	log.Infof("setting new log spec: %s", string(body))
	err = ConfigureLoggers(string(body))
	if err != nil {
		c.AbortWithStatus(400)
		return
	}

	c.String(200, LoggerInfo())
}
