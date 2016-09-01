// +build race

package rex

import (
	"sync"
	"time"

	"github.com/jinzhu/now"
)

// to reduce allocations and calls to time.Now
var timeToday time.Time
var metricsTimestampNow = ""
var metricsTimestampNowUrlSafe = ""
var metricsTimestampMutex = sync.RWMutex{}

func updateMetricsTimestampNow() {
	for {
		time.Sleep(1 * time.Second)
		metricsTimestampMutex.Lock()
		today = now.BeginningOfDay()
		metricsTimestampNow = time.Now().UTC().Format("2006-01-02T15:04:05Z")
		metricsTimestampNowUrlSafe = time.Now().UTC().Format("2006-01-02T15-04-05Z")
		metricsTimestampMutex.Unlock()
	}
}

func init() {
	go updateMetricsTimestampNow()
}

func GetToday() time.Time {
	metricsTimestampMutex.RLock()
	defer metricsTimestampMutex.RUnlock()
	return timeToday
}

func GetTimestampNowFormat() string {
	metricsTimestampMutex.RLock()
	defer metricsTimestampMutex.RUnlock()
	return metricsTimestampNow
}

func GetTimestampNowUrlSafe() string {
	metricsTimestampMutex.RLock()
	defer metricsTimestampMutex.RUnlock()
	return metricsTimestampNowUrlSafe
}
