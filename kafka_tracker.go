package rex

import (
	"github.com/Shopify/sarama"
	"github.com/juju/loggo"
)

type KafkaTracker struct {
	BaseTracker
	Log      loggo.Logger
	Client   *sarama.Client
	Producer *sarama.Producer
}

func NewKafkaTracker(config *Config) Tracker {
	self := &KafkaTracker{}
	self.EventMetadata = &config.EventMetadata
	self.Log = loggo.GetLogger("rex.tracker")

	client, err := NewKafkaClient(self.Service, config.KafkaBroker, nil)
	MayPanic(err)

	producer, err := NewKafkaProducer(client, nil)
	if err != nil {
		client.Close()
		MayPanic(err)
	}

	self.Log.Infof("connected")
	self.Client = client
	self.Producer = producer

	return self
}

func (self *KafkaTracker) Close() {
	self.Log.Debugf("shutting down tracker")
	CaptureError(self.Producer.Close())
	CaptureError(self.Client.Close())
	self.Log.Debugf("tracker is stopped")
}

func (self *KafkaTracker) Message(topic string, message []byte) {
	self.Log.Tracef("trying to send message: %s", string(message))
	if message == nil || len(message) < 1 {
		CaptureErrorNew("empty message")
		return
	}

	self.Producer.Input() <- &sarama.MessageToSend{
		Topic: topic,
		Value: sarama.ByteEncoder(message),
	}

	select {
	case msg := <-self.Producer.Errors():
		CaptureError(msg.Err)
	case msg := <-self.Producer.Successes():
		value, _ := msg.Value.Encode()
		self.Log.Tracef("successfully sent message: %s", string(value))
	}
}

func (self *KafkaTracker) Event(topic string, e EventBase, full bool) {
	self.AddMetadata(e, full)
	go self.Message(topic, self.Encode(e))
}

func (self *KafkaTracker) EventMap(topic string, event map[string]interface{}, full bool) {
	self.AddMetadataMap(event, full)
	go self.Message(topic, self.Encode(event))
}
