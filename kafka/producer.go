package kafka

import (
	"errors"
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
			rollbar.Error(rollbar.ERR, err)
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

func (client *Client) NewFastProducer(cb ProducerErrorCallback) (*Producer, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = false
	config.Producer.Return.Errors = true
	config.Producer.RequiredAcks = sarama.NoResponse
	if os.Getenv("REX_ENV") != "development" {
		config.Producer.Flush.Frequency = 1 * time.Second
	}
	return client.NewProducer("fast", config, cb)
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
	self.Close()
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

	select {
	case self.Input() <- msg:
	default:
		self.errors.Update(time.Since(start))
		return errors.New("input would block")
	}

	// fast producer doesn't care after input
	if self.config.Producer.Return.Successes == false {
		self.messages.Update(time.Since(start))
		return nil
	}

	// safe producer waits for response
	select {
	case err := <-self.Errors():
		self.errors.Update(time.Since(start))
		return err.Err
	case <-self.Successes():
		self.messages.Update(time.Since(start))
	}

	return nil
}
