// +build !race

package rex

import (
	"time"

	"github.com/jinzhu/now"
)

// to reduce allocations and calls to time.Now
var timeToday time.Time
var metricsTimestampNow = ""
var metricsTimestampNowUrlSafe = ""

func updateMetricsTimestampNow() {
	for {
		timeToday = now.BeginningOfDay()
		metricsTimestampNow = time.Now().UTC().Format("2006-01-02T15:04:05Z")
		metricsTimestampNowUrlSafe = time.Now().UTC().Format("2006-01-02T15-04-05Z")
		time.Sleep(1 * time.Second)
	}
}

func init() {
	go updateMetricsTimestampNow()
}

func GetToday() time.Time {
	return timeToday
}

func GetTimestampNowFormat() string {
	return metricsTimestampNow
}

func GetTimestampNowUrlSafe() string {
	return metricsTimestampNowUrlSafe
}
