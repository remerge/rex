package rex

import (
	"time"

	"github.com/Shopify/sarama"
	"github.com/juju/loggo"
	"github.com/remerge/rex/kafka"
	"github.com/remerge/rex/log"
	"github.com/remerge/rex/rollbar"
)

type KafkaTracker struct {
	*BaseTracker
	Client      *kafka.Client
	Fast        *kafka.Producer
	FastTimeout time.Duration
	Safe        *kafka.Producer
	SafeTimeout time.Duration
	log         loggo.Logger
}

func NewKafkaTracker(broker string, metadata *EventMetadata) (_ Tracker, err error) {
	self := &KafkaTracker{
		BaseTracker: NewBaseTracker(metadata),
		FastTimeout: 10 * time.Millisecond,
		SafeTimeout: 100 * time.Millisecond,
		log:         log.GetLogger("rex.tracker"),
	}

	self.Client, err = kafka.NewClient("tracker", broker)
	if err != nil {
		return nil, err
	}

	self.Fast, err = self.Client.NewFastProducer(nil)
	if err != nil {
		self.Close()
		return nil, err
	}

	self.Safe, err = self.Client.NewSafeProducer()
	if err != nil {
		self.Close()
		return nil, err
	}

	return self, nil
}

func (self *KafkaTracker) Close() {
	if self.Safe != nil {
		self.log.Infof("shutting down safe producer")
		self.Safe.Shutdown()
	}
	if self.Fast != nil {
		self.log.Infof("shutting down fast producer")
		self.Fast.Shutdown()
	}
	rollbar.Error(rollbar.WARN, self.Client.Close())
	self.log.Infof("tracker is stopped")
}

func (self *KafkaTracker) FastMessage(topic string, value []byte) error {
	if self.log.IsTraceEnabled() {
		self.log.Tracef("topic=%s value=%s", topic, string(value))
	}
	return self.Fast.SendMessage(&sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(value),
	})
}

func (self *KafkaTracker) FastEvent(topic string, e EventBase, full bool) error {
	self.AddMetadata(e, full)
	return self.FastMessage(topic, self.Encode(e))
}

func (self *KafkaTracker) FastEventMap(topic string, event map[string]interface{}, full bool) error {
	self.AddMetadataMap(event, full)
	return self.FastMessage(topic, self.EncodeMap(event))
}

func (self *KafkaTracker) SafeMessage(topic string, value []byte) error {
	if self.log.IsTraceEnabled() {
		self.log.Tracef("topic=%s value=%s", topic, string(value))
	}
	return self.Safe.SendMessage(&sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(value),
	})
}

func (self *KafkaTracker) SafeEvent(topic string, event EventBase, full bool) error {
	self.AddMetadata(event, full)
	return self.SafeMessage(topic, self.Encode(event))
}

func (self *KafkaTracker) SafeEventMap(topic string, event map[string]interface{}, full bool) error {
	self.AddMetadataMap(event, full)
	return self.SafeMessage(topic, self.EncodeMap(event))
}
