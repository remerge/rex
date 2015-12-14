package rex

import (
	"encoding/json"
)

type BaseTracker struct {
	*EventMetadata
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
		event.Ts = GetTimestampNowFormat()

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
		event["ts"] = GetTimestampNowFormat()

		if full == true {
			event["service"] = self.Service
			event["env"] = self.Environment
			event["cluster"] = self.Cluster
			event["host"] = self.Host
			event["release"] = self.Release
		}
	}
}
