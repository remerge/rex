package kafka

import (
	"fmt"
	"os"
	"time"

	"github.com/Shopify/sarama"
	"github.com/heroku/instruments"
	"github.com/heroku/instruments/reporter"
	"github.com/juju/loggo"
	"github.com/remerge/rex/rollbar"
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
	messages *instruments.Timer
	errors   *instruments.Timer
}

func (client *Client) NewProducer(name string, config *sarama.Config, cb ProducerErrorCallback) (self *Producer, err error) {
	name = fmt.Sprintf("kafka.producer.%s.%s", client.GetId(), name)

	if cb == nil {
		cb = func(err *sarama.ProducerError) {
			loggo.GetLogger(name).Errorf("%v", err)
		}
	}

	self = &Producer{
		callback: cb,
		config:   config,
		quit:     make(chan bool),
		done:     make(chan bool),
		log:      loggo.GetLogger(name),
		messages: reporter.NewRegisteredTimer(name+".messages", -1),
		errors:   reporter.NewRegisteredTimer(name+".errors", -1),
	}

	if config == nil {
		self.AsyncProducer, err = sarama.NewAsyncProducerFromClient(client)
	} else {
		self.AsyncProducer, err = sarama.NewAsyncProducer(client.brokers, config)
	}

	if err != nil {
		return nil, err
	}

	go self.Start()

	return self, nil
}

func (client *Client) newFastProducer(name string, config *sarama.Config, cb ProducerErrorCallback) (*Producer, error) {
	config.Producer.Return.Successes = false
	config.Producer.Return.Errors = true
	config.Producer.RequiredAcks = sarama.NoResponse
	config.ChannelBufferSize = 131072 // buffer 128k messages
	if os.Getenv("REX_ENV") != "development" {
		config.Producer.Flush.Frequency = 1 * time.Second
	}
	return client.NewProducer(name, config, cb)
}

func (client *Client) NewCompressingFastProducer(cb ProducerErrorCallback) (*Producer, error) {
	config := sarama.NewConfig()
	config.Producer.Compression = sarama.CompressionSnappy
	return client.newFastProducer("fast_compressing", config, cb)
}

func (client *Client) NewFastProducer(cb ProducerErrorCallback) (*Producer, error) {
	return client.newFastProducer("fast", sarama.NewConfig(), cb)
}

func (client *Client) NewSafeProducer() (*Producer, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Producer.RequiredAcks = sarama.WaitForLocal
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
			if ok {
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
		self.log.Infof("shutting down producer error loop")
		close(self.quit)
		self.log.Infof("waiting for error loop to finish")
		<-self.done
	}
	self.log.Infof("closing producer")
	rollbar.Error(rollbar.WARN, self.Close())
	self.log.Infof("shutdown done")
}

func (self *Producer) SendMessage(msg *sarama.ProducerMessage) (err error) {
	start := time.Now()

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

	self.Input() <- msg

	// safe producer waits for response
	if self.config.Producer.Return.Successes == true {
		select {
		case err := <-self.Errors():
			self.errors.Update(time.Since(start))
			return err.Err
		case <-self.Successes():
			// do nothing
		}
	}

	self.messages.Update(time.Since(start))
	return nil
}
