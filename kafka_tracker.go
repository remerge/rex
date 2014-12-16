package rex

import (
	"encoding/gob"
	"time"

	"github.com/Shopify/sarama"
	"github.com/juju/loggo"
)

type KafkaTracker struct {
	*BaseTracker
	log      loggo.Logger
	Client   *sarama.Client
	producer *sarama.Producer
	queue    *DiskQueue
	backoff  time.Duration
	quit     chan bool
	done     chan bool
}

func NewKafkaTracker(name string, broker string, metadata *EventMetadata) (_ Tracker, err error) {
	self := &KafkaTracker{
		BaseTracker: NewBaseTracker(metadata),
		log:         loggo.GetLogger("rex.tracker." + name),
		queue:       NewDiskQueue("tracker_"+name, "cache", 128*1024*1024, 5000, 1*time.Second),
		backoff:     10 * time.Millisecond,
		quit:        make(chan bool),
		done:        make(chan bool),
	}

	self.Client, err = NewKafkaClient(name, broker, nil)
	if err != nil {
		CaptureError(err)
		return nil, err
	}

	self.producer, err = NewKafkaProducer(self.Client, nil)
	if err != nil {
		self.Client.Close()
		CaptureError(err)
		return nil, err
	}

	self.log.Infof("connected")
	go self.start()

	return self, nil
}

func (self *KafkaTracker) Close() {
	self.log.Infof("shutting down tracker")
	close(self.quit)
	self.log.Infof("waiting for run loop")
	<-self.done
	CaptureError(self.producer.Close())
	CaptureError(self.Client.Close())
	CaptureError(self.queue.Close())
	self.log.Infof("tracker is stopped")
}

func init() {
	gob.Register(sarama.MessageToSend{})
	gob.Register(sarama.ByteEncoder{})
}

func (self *KafkaTracker) Message(topic string, message []byte) error {
	self.log.Tracef("enqueue topic=%v message=%v", topic, string(message))
	if message == nil || len(message) < 1 {
		MayPanicNew("empty message")
	}

	go func() {
		CaptureError(self.enqueue(&sarama.MessageToSend{
			Topic: topic,
			Value: sarama.ByteEncoder(message),
		}))
	}()

	return nil
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
			self.log.Tracef("trying to send message: %s", string(value))
			select {
			case self.producer.Input() <- &msg:
				self.backoff = 10 * time.Millisecond
			default:
				self.log.Tracef("producer input would block message (backoff=%v): %s", self.backoff, string(value))
				time.Sleep(self.backoff)
				self.backoff = 2 * self.backoff
				CaptureError(self.enqueue(&msg))
			}
		case err := <-self.producer.Errors():
			value, _ := err.Msg.Value.Encode()
			self.log.Tracef("failed to send message: %s", string(value))
			CaptureError(err.Err)
			CaptureError(self.enqueue(err.Msg))
		case msg := <-self.producer.Successes():
			value, _ := msg.Value.Encode()
			self.log.Tracef("successfully sent message: %s", string(value))
		case <-self.quit:
			close(self.done)
			return
		}
	}
}

func (self *KafkaTracker) Event(topic string, e EventBase, full bool) error {
	self.AddMetadata(e, full)
	return self.Message(topic, self.Encode(e))
}

func (self *KafkaTracker) EventMap(topic string, event map[string]interface{}, full bool) error {
	self.AddMetadataMap(event, full)
	return self.Message(topic, self.Encode(event))
}
