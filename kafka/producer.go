package kafka

import (
	"errors"
	"fmt"
	"time"

	"github.com/Shopify/sarama"
	"github.com/juju/loggo"
	"github.com/rcrowley/go-metrics"
)

type ProducerErrorCallback func(*sarama.ProducerError)

type Producer struct {
	sarama.AsyncProducer
	config   *sarama.Config
	callback ProducerErrorCallback
	running  bool
	quit     chan bool
	done     chan bool
	log      loggo.Logger
	messages metrics.Timer
	errors   metrics.Timer
}

func (client *Client) NewProducer(name string, config *sarama.Config, cb ProducerErrorCallback) (*Producer, error) {
	name = fmt.Sprintf("kafka.producer.%s.%s", client.GetId(), name)

	self := &Producer{
		callback: cb,
		config:   config,
		quit:     make(chan bool),
		done:     make(chan bool),
		log:      loggo.GetLogger(name),
		messages: metrics.NewRegisteredTimer(name+".messages", metrics.DefaultRegistry),
		errors:   metrics.NewRegisteredTimer(name+".errors", metrics.DefaultRegistry),
	}

	producer, err := sarama.NewAsyncProducer(client.brokers, config)
	if err != nil {
		self.log.Errorf("failed to create producer: %s", err)
		return nil, err
	}

	self.AsyncProducer = producer
	go self.Start()

	return self, nil
}

func (client *Client) NewFastProducer(cb ProducerErrorCallback) (*Producer, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = false
	config.Producer.RequiredAcks = sarama.NoResponse
	config.Producer.Flush.Messages = 10000
	config.Producer.Flush.Frequency = 10 * time.Millisecond
	return client.NewProducer("fast", config, cb)
}

func (client *Client) NewSafeProducer() (*Producer, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Timeout = 100 * time.Millisecond
	return client.NewProducer("safe", config, nil)
}

func (self *Producer) Start() {
	if self.config.Producer.Return.Successes == true {
		return
	}

	self.running = true
	for {
		select {
		case err, ok := <-self.Errors():
			if ok && self.callback != nil {
				self.callback(err)
			}
		case <-self.quit:
			self.running = false
			close(self.done)
			return
		}
	}
}

func (self *Producer) Shutdown() {
	if self.running {
		self.log.Infof("shutting down producer run loop")
		close(self.quit)
		self.log.Infof("waiting for run loop to finish")
		<-self.done
	}
	self.log.Infof("closing producer")
	self.Close()
	self.log.Infof("shutdown done")
}

func (self *Producer) Message(topic string, value []byte, timeout time.Duration) (msg *sarama.ProducerMessage, err error) {
	start := time.Now()

	if value == nil || len(value) < 1 {
		self.errors.Update(time.Since(start))
		return msg, errors.New("empty message")
	}

	msg = &sarama.ProducerMessage{
		Topic: string(topic),
		Value: sarama.ByteEncoder(value),
	}

	defer func() {
		// we might have tried to write to a closed channel during shutdown
		// recover and return the error to the caller
		if r := recover(); r != nil {
			var ok bool
			err, ok = r.(error)
			if !ok {
				err = fmt.Errorf("unknown error: %v", r)
			}
			self.errors.Update(time.Since(start))
		}
	}()

	after := time.After(timeout)

	select {
	case self.Input() <- msg:
		// fall through
	case <-after:
		self.errors.Update(time.Since(start))
		return msg, errors.New("input timed out")
	}

	if self.config.Producer.Return.Successes == false {
		self.messages.Update(time.Since(start))
		return msg, nil
	}

	select {
	case err := <-self.Errors():
		self.errors.Update(time.Since(start))
		return msg, err.Err
	case <-self.Successes():
		self.messages.Update(time.Since(start))
	case <-after:
		self.errors.Update(time.Since(start))
		return msg, errors.New("ack timed out")
	}

	return msg, nil
}
