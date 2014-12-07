package rex

type Tracker interface {
	Close()
	Message(topic string, message []byte) error
	Event(topic string, event EventBase, metadata bool) error
	EventMap(topic string, event map[string]interface{}, metadata bool) error
}
