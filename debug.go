package rex

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/gin-gonic/gin"
	. "github.com/remerge/rex/log"
	"github.com/remerge/rex/rollbar"
)

func Inspect(v interface{}) string {
	bytes, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%#v", v)
	}
	return string(bytes)
}

func getLoggoSpec(c *gin.Context) {
	c.String(200, LoggerInfo())
}

func setLoggoSpec(c *gin.Context) {
	log := GetLogger("rex.debug")

	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		rollbar.Error(rollbar.WARN, err)
		c.AbortWithStatus(500)
		return
	}

	log.Infof("setting new log spec: %s", string(body))
	err = ConfigureLoggers(string(body))
	if err != nil {
		rollbar.Error(rollbar.WARN, err)
		c.AbortWithStatus(400)
		return
	}

	c.String(200, LoggerInfo())
}
