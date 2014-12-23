package kafka

import (
	"fmt"

	"github.com/Shopify/sarama"
	"github.com/juju/loggo"
)

type Consumer struct {
	*sarama.Consumer
	quit chan bool
	done chan bool
	log  loggo.Logger
}

func (client *Client) NewConsumer(group string, topic string, partition int32, config *sarama.ConsumerConfig) (*Consumer, error) {
	name := fmt.Sprintf("kafka.consumer.%s.%s.%s.%d", client.GetId(), group, topic, partition)

	self := &Consumer{
		quit: make(chan bool),
		done: make(chan bool),
		log:  loggo.GetLogger(name),
	}

	if config == nil {
		self.log.Infof("using default producer config")
		config = sarama.NewConsumerConfig()
	}

	consumer, err := sarama.NewConsumer(client.Client, topic, partition, group, config)
	if err != nil {
		self.log.Errorf("failed to create consumer: %s", err)
		return nil, err
	}

	self.Consumer = consumer
	return self, nil

}

func (self *Consumer) Start(events chan *sarama.ConsumerEvent) {
	for {
		select {
		case event := <-self.Events():
			events <- event
		case <-self.quit:
			close(self.done)
			return
		}
	}
}

func (self *Consumer) Shutdown() {
	self.log.Infof("shutting down consumer run loop")
	close(self.quit)
	self.log.Infof("waiting for run loop to finish")
	<-self.done
	self.log.Infof("closing consumer")
	self.Close()
	self.log.Infof("shutdown done")
}
