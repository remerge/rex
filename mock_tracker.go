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

func (self *MockTracker) FastMessage(topic string, message []byte) {
	self.Messages[topic] = append(self.Messages[topic], message)
}

func (self *MockTracker) FastEvent(topic string, e EventBase, full bool) {
	self.AddMetadata(e, full)
	self.FastMessage(topic, self.Encode(e))
}

func (self *MockTracker) FastEventMap(topic string, event map[string]interface{}, full bool) {
	self.AddMetadataMap(event, full)
	self.FastMessage(topic, self.EncodeMap(event))
}

func (self *MockTracker) SafeMessage(topic string, message []byte) {
	self.Messages[topic] = append(self.Messages[topic], message)
}

func (self *MockTracker) SafeEvent(topic string, e EventBase, full bool) {
	self.AddMetadata(e, full)
	self.SafeMessage(topic, self.Encode(e))
}

func (self *MockTracker) SafeEventMap(topic string, event map[string]interface{}, full bool) {
	self.AddMetadataMap(event, full)
	self.SafeMessage(topic, self.EncodeMap(event))
}
