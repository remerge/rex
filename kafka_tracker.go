package rex

import (
	"encoding/gob"
	"time"

	"github.com/Shopify/sarama"
	"github.com/juju/loggo"
	"github.com/remerge/rex/kafka"
)

type KafkaTracker struct {
	*BaseTracker
	log         loggo.Logger
	Client      *kafka.Client
	Fast        *kafka.Producer
	FastTimeout time.Duration
	Safe        *kafka.Producer
	SafeTimeout time.Duration
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
		queue:       NewDiskQueue("tracker", "cache", 128*1024*1024, 5000, 1*time.Second),
		quit:        make(chan bool),
		done:        make(chan bool),
	}

	self.Client, err = kafka.NewClient("tracker", broker, nil)
	if err != nil {
		CaptureError(err)
		return nil, err
	}

	self.Fast, err = self.Client.NewFastProducer(self.ErrorHandler)
	if err != nil {
		CaptureError(err)
		self.Close()
		return nil, err
	}

	self.Safe, err = self.Client.NewSafeProducer()
	if err != nil {
		CaptureError(err)
		self.Close()
		return nil, err
	}

	go self.start()

	return self, nil
}

func (self *KafkaTracker) Close() {
	self.log.Infof("shutting down tracker")
	close(self.quit)
	self.log.Infof("waiting for run loop")
	<-self.done
	if self.Safe != nil {
		self.log.Infof("shutting down safe producer")
		self.Safe.Shutdown()
	}
	if self.Fast != nil {
		self.log.Infof("shutting down fast producer")
		self.Fast.Shutdown()
	}
	CaptureError(self.Client.Close())
	CaptureError(self.queue.Close())
	self.log.Infof("tracker is stopped")
}

// fast async producer with no ack

func (self *KafkaTracker) ErrorHandler(err *sarama.ProduceError) {
}

func (self *KafkaTracker) FastMessage(topic string, value []byte) {
	self.log.Tracef("topic=%s value=%s", topic, string(value))
	self.Fast.Message(topic, value, self.FastTimeout)
}

func (self *KafkaTracker) FastEvent(topic string, e EventBase, full bool) {
	self.AddMetadata(e, full)
	self.FastMessage(topic, self.Encode(e))
}

func (self *KafkaTracker) FastEventMap(topic string, event map[string]interface{}, full bool) {
	self.AddMetadataMap(event, full)
	self.FastMessage(topic, self.Encode(event))
}

// slower but reliable WaitForAll producer with ack

func (self *KafkaTracker) SafeMessage(topic string, value []byte) {
	self.log.Tracef("topic=%s value=%s", topic, string(value))
	msg, err := self.Safe.Message(topic, value, self.SafeTimeout)
	if err != nil {
		self.enqueue(msg)
	}
}

func (self *KafkaTracker) SafeEvent(topic string, e EventBase, full bool) {
	self.AddMetadata(e, full)
	self.SafeMessage(topic, self.Encode(e))
}

func (self *KafkaTracker) SafeEventMap(topic string, event map[string]interface{}, full bool) {
	self.AddMetadataMap(event, full)
	self.SafeMessage(topic, self.Encode(event))
}

// fail-safe disk queue worker

func init() {
	gob.Register(sarama.MessageToSend{})
	gob.Register(sarama.ByteEncoder{})
}

func (self *KafkaTracker) enqueue(msg *sarama.MessageToSend) error {
	bytes, err := GobEncode(msg)
	if err != nil {
		return err
	}

	err = self.queue.Put(bytes)
	if err != nil {
		return err
	}

	return nil
}

func (self *KafkaTracker) start() {
	for {
		select {
		case bytes := <-self.queue.ReadChan():
			i, err := GobDecode(bytes)
			if err != nil {
				CaptureError(err)
				continue
			}
			msg := i.(sarama.MessageToSend)
			value, _ := msg.Value.Encode()
			self.SafeMessage(msg.Topic, value)
		case <-self.quit:
			close(self.done)
			return
		}
	}
}
