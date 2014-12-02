package rex

import (
	"errors"

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

	pconfig := sarama.NewProducerConfig()

	if self.Environment != "production" {
		pconfig.RequiredAcks = sarama.WaitForAll
	}

	producer, err := NewKafkaProducer(client, pconfig)
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
	self.Log.Infof("shutting down tracker")
	self.Client.Close()
}

func (self *KafkaTracker) Message(topic string, message []byte) error {
	self.Log.Tracef("event %s", string(message))
	if message == nil {
		return errors.New("empty message")
	}
	if self.Environment != "production" {
		return self.Producer.SendMessage(topic, nil, sarama.ByteEncoder(message))
	}
	return self.Producer.QueueMessage(topic, nil, sarama.ByteEncoder(message))
}

func (self *KafkaTracker) Event(topic string, e EventBase, full bool) error {
	self.AddMetadata(e, full)
	return self.Message(topic, self.Encode(e))
}

func (self *KafkaTracker) EventMap(topic string, event map[string]interface{}, full bool) error {
	self.AddMetadataMap(event, full)
	return self.Message(topic, self.Encode(event))
}
