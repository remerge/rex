package rex

type Tracker interface {
	Close()
	FastMessage(topic string, message []byte) error
	FastEvent(topic string, event EventBase, metadata bool) error
	FastEventMap(topic string, event map[string]interface{}, metadata bool) error
	SafeMessage(topic string, message []byte) error
	SafeEvent(topic string, event EventBase, metadata bool) error
	SafeEventMap(topic string, event map[string]interface{}, metadata bool) error
}
