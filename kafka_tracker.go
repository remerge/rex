package rex

import (
	"bytes"
	"time"

	"github.com/Shopify/sarama"
	"github.com/juju/loggo"
	"github.com/remerge/rex/kafka"
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
	running     bool
	quit        chan bool
	done        chan bool
	queue       *DiskQueue
}

func NewKafkaTracker(broker string, metadata *EventMetadata) (_ Tracker, err error) {
	self := &KafkaTracker{
		BaseTracker: NewBaseTracker(metadata),
		FastTimeout: 10 * time.Millisecond,
		SafeTimeout: 100 * time.Millisecond,
		log:         loggo.GetLogger("rex.tracker"),
		queue:       NewDiskQueue("tracker.v2", "cache", 128*1024*1024, 5000, 1*time.Second),
		quit:        make(chan bool),
		done:        make(chan bool),
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

	go self.start()

	return self, nil
}

func (self *KafkaTracker) Close() {
	if self.running {
		self.log.Infof("shutting down tracker")
		close(self.quit)
		self.log.Infof("waiting for run loop")
		<-self.done
	}
	if self.Safe != nil {
		self.log.Infof("shutting down safe producer")
		self.Safe.Shutdown()
	}
	if self.Fast != nil {
		self.log.Infof("shutting down fast producer")
		self.Fast.Shutdown()
	}
	rollbar.Error(rollbar.WARN, self.Client.Close())
	rollbar.Error(rollbar.WARN, self.queue.Close())
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
	return self.enqueue(topic, value)
}

func (self *KafkaTracker) SafeEvent(topic string, event EventBase, full bool) error {
	self.AddMetadata(event, full)
	return self.SafeMessage(topic, self.Encode(event))
}

func (self *KafkaTracker) SafeEventMap(topic string, event map[string]interface{}, full bool) error {
	self.AddMetadataMap(event, full)
	return self.SafeMessage(topic, self.EncodeMap(event))
}

// fail-safe disk queue worker

var safeQueueDelim = []byte{0x0}

func (self *KafkaTracker) enqueue(topic string, value []byte) error {
	msg := append([]byte(topic), safeQueueDelim...)
	msg = append(msg, value...)
	return self.queue.Put(msg)
}

func (self *KafkaTracker) processSafeMessage(msg []byte) error {
	idx := bytes.Index(msg, safeQueueDelim)
	topic := string(msg[0:idx])
	value := msg[idx+1:]

	return self.Safe.SendMessage(&sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(value),
	})
}

func (self *KafkaTracker) start() {
	self.running = true

	for {
		select {
		case bytes := <-self.queue.ReadChan():
			if err := self.processSafeMessage(bytes); err != nil {
				rollbar.Error(rollbar.ERR, err)
				self.queue.Put(bytes)
			}
		case <-self.quit:
			self.running = false
			close(self.done)
			return
		}
	}
}
