// +build !race

package rex

import "time"

// to reduce allocations in AddMetadata
var metricsTimestampNow = ""

func updateMetricsTimestampNow() {
	for {
		metricsTimestampNow = time.Now().UTC().Format("2006-01-02T15:04:05Z")
		time.Sleep(1 * time.Second)
	}
}

func init() {
	go updateMetricsTimestampNow()
}

func GetTimestampNowFormat() string {
	return metricsTimestampNow
}
