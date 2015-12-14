// +build race

package rex

import (
	"sync"
	"time"
)

// to reduce allocations in AddMetadata
var metricsTimestampNow = ""
var metricsTimestampMutex = sync.Mutex{}

func updateMetricsTimestampNow() {
	for {
		time.Sleep(1 * time.Second)
		metricsTimestampMutex.Lock()
		defer metricsTimestampMutex.Unlock()
		metricsTimestampNow = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	}
}

func init() {
	go updateMetricsTimestampNow()
}

func GetTimestampNowFormat() string {
	metricsTimestampMutex.Lock()
	defer metricsTimestampMutex.Unlock()
	return metricsTimestampNow
}
