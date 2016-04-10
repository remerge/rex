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

func (self *MockTracker) FastMessage(topic string, message []byte) error {
	self.Messages[topic] = append(self.Messages[topic], message)
	return nil
}

func (self *MockTracker) FastEvent(topic string, e EventBase, full bool) error {
	self.AddMetadata(e, full)
	return self.FastMessage(topic, self.Encode(e))
}

func (self *MockTracker) FastEventMap(topic string, event map[string]interface{}, full bool) error {
	self.AddMetadataMap(event, full)
	return self.FastMessage(topic, self.EncodeMap(event))
}

func (self *MockTracker) SafeMessage(topic string, message []byte) error {
	self.Messages[topic] = append(self.Messages[topic], message)
	return nil
}

func (self *MockTracker) SafeEvent(topic string, e EventBase, full bool) error {
	self.AddMetadata(e, full)
	return self.SafeMessage(topic, self.Encode(e))
}

func (self *MockTracker) SafeEventMap(topic string, event map[string]interface{}, full bool) error {
	self.AddMetadataMap(event, full)
	return self.SafeMessage(topic, self.EncodeMap(event))
}
