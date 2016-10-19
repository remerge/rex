package rollbar

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

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

			for _, err := range c.Errors {
				RequestErrorWithStackSkip(ERR, c.Request, err, 3)
			}

			c.JSON(500, gin.H{
				"errors": c.Errors,
			})
		}()

		c.Next()
	}
}
