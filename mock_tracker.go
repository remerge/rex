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

func (self *MockTracker) Message(topic string, message []byte) error {
	self.Messages[topic] = append(self.Messages[topic], message)
	return nil
}

func (self *MockTracker) Event(topic string, e EventBase, full bool) error {
	self.AddMetadata(e, full)
	return self.Message(topic, self.Encode(e))
}

func (self *MockTracker) EventMap(topic string, event map[string]interface{}, full bool) error {
	self.AddMetadataMap(event, full)
	return self.Message(topic, self.Encode(event))
}
