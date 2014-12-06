package rex

type MockTracker struct {
	BaseTracker
	Messages map[string][][]byte
}

func NewMockTracker(config *Config) *MockTracker {
	self := &MockTracker{}
	self.EventMetadata = &config.EventMetadata
	self.Messages = make(map[string][][]byte)
	return self
}

func (self *MockTracker) Close() {
}

func (self *MockTracker) Message(topic string, message []byte) {
	self.Messages[topic] = append(self.Messages[topic], message)
}

func (self *MockTracker) Event(topic string, e EventBase, full bool) {
	self.AddMetadata(e, full)
	self.Message(topic, self.Encode(e))
}

func (self *MockTracker) EventMap(topic string, event map[string]interface{}, full bool) {
	self.AddMetadataMap(event, full)
	self.Message(topic, self.Encode(event))
}
