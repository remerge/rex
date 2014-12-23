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
	log    loggo.Logger
	Client *kafka.Client
	Fast   *kafka.Producer
	Sync   *kafka.Producer
	Safe   *kafka.Producer
	quit   chan bool
	done   chan bool
	queue  *DiskQueue
}

func NewKafkaTracker(service string, broker string, metadata *EventMetadata) (_ Tracker, err error) {
	self := &KafkaTracker{
		BaseTracker: NewBaseTracker(metadata),
		log:         loggo.GetLogger("rex.tracker"),
		queue:       NewDiskQueue("tracker_"+service, "cache", 128*1024*1024, 5000, 1*time.Second),
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

	self.Sync, err = self.Client.NewSyncProducer()
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
	if self.Sync != nil {
		self.log.Infof("shutting down sync producer")
		self.Sync.Shutdown()
	}
	if self.Fast != nil {
		self.log.Infof("shutting down fast producer")
		self.Fast.Shutdown()
	}
	CaptureError(self.Client.Close())
	CaptureError(self.queue.Close())
	self.log.Infof("tracker is stopped")
}

func init() {
	gob.Register(sarama.MessageToSend{})
	gob.Register(sarama.ByteEncoder{})
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
			self.Fast.Input() <- &msg
		case <-self.quit:
			close(self.done)
			return
		}
	}
}

func (self *KafkaTracker) ErrorHandler(err *sarama.ProduceError) {
	CaptureError(err.Err)
}

func (self *KafkaTracker) FastMessage(topic string, value []byte) error {
	self.log.Tracef("topic=%s value=%s", topic, string(value))
	return self.Fast.Message(topic, value, 1*time.Millisecond)
}

func (self *KafkaTracker) FastEvent(topic string, e EventBase, full bool) error {
	self.AddMetadata(e, full)
	return self.FastMessage(topic, self.Encode(e))
}

func (self *KafkaTracker) FastEventMap(topic string, event map[string]interface{}, full bool) error {
	self.AddMetadataMap(event, full)
	return self.FastMessage(topic, self.Encode(event))
}

func (self *KafkaTracker) SafeMessage(topic string, value []byte) error {
	self.log.Tracef("topic=%s value=%s", topic, string(value))
	return self.Safe.Message(topic, value, 10*time.Millisecond)
}

func (self *KafkaTracker) SafeEvent(topic string, e EventBase, full bool) error {
	self.AddMetadata(e, full)
	return self.SafeMessage(topic, self.Encode(e))
}

func (self *KafkaTracker) SafeEventMap(topic string, event map[string]interface{}, full bool) error {
	self.AddMetadataMap(event, full)
	return self.SafeMessage(topic, self.Encode(event))
}
