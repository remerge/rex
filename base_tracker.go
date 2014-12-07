package rex

import (
	"encoding/json"

	"time"
)

type BaseTracker struct {
	*EventMetadata
}

func (self *BaseTracker) Encode(message interface{}) []byte {
	bytes, _ := json.Marshal(message)
	return bytes
}

func (self *BaseTracker) AddMetadata(e EventBase, full bool) {
	event := e.Base()

	if event.Ts == "" {
		event.Ts = time.Now().UTC().Format("2006-01-02T15:04:05Z")

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
		event["ts"] = time.Now().UTC().Format("2006-01-02T15:04:05Z")

		if full == true {
			event["service"] = self.Service
			event["env"] = self.Environment
			event["cluster"] = self.Cluster
			event["host"] = self.Host
			event["release"] = self.Release
		}
	}
}
