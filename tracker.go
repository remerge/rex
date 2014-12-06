package rex

type Tracker interface {
	Close()
	Message(topic string, message []byte)
	Event(topic string, event EventBase, metadata bool)
	EventMap(topic string, event map[string]interface{}, metadata bool)
}
