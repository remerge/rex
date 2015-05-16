package rex

type Tracker interface {
	Close()
	AddMetadata(e EventBase, full bool)
	FastMessage(topic string, message []byte)
	FastEvent(topic string, event EventBase, metadata bool)
	FastEventMap(topic string, event map[string]interface{}, metadata bool)
	SafeMessage(topic string, message []byte)
	SafeEvent(topic string, event EventBase, metadata bool)
	SafeEventMap(topic string, event map[string]interface{}, metadata bool)
}
