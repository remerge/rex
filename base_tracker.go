package rex

import (
	"encoding/json"
	"time"
)

type BaseTracker struct {
	*EventMetadata
}

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

func NewBaseTracker(metadata *EventMetadata) *BaseTracker {
	return &BaseTracker{
		EventMetadata: metadata,
	}
}

func (self *BaseTracker) Encode(event EventBase) []byte {
	bytes, _ := event.MarshalJSON()
	return bytes
}

func (self *BaseTracker) EncodeMap(event map[string]interface{}) []byte {
	bytes, _ := json.Marshal(event)
	return bytes
}

func (self *BaseTracker) AddMetadata(e EventBase, full bool) {
	event := e.Base()

	if event.Ts == "" {
		event.Ts = metricsTimestampNow

		if full == true {
			event.Service = self.Service
			event.Environment = self.Environment
			event.Cluster = self.Cluster
			event.Host = self.Host
			event.Release = self.Release
		}
	}
}

func (self *BaseTracker) AddMetadataMap(event map[string]interface{}, full bool) {
	if event["ts"] == nil {
		event["ts"] = metricsTimestampNow

		if full == true {
			event["service"] = self.Service
			event["env"] = self.Environment
			event["cluster"] = self.Cluster
			event["host"] = self.Host
			event["release"] = self.Release
		}
	}
}
