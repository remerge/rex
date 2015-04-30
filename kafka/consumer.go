package kafka

import (
	"fmt"

	"github.com/Shopify/sarama"
	"github.com/juju/loggo"
)

type Consumer struct {
	sarama.PartitionConsumer
	master  sarama.Consumer
	running bool
	quit    chan bool
	done    chan bool
	log     loggo.Logger
}

func (client *Client) NewConsumer(topic string, partition int32, offset int64, config *sarama.Config) (self *Consumer, err error) {
	name := fmt.Sprintf("kafka.consumer.%s.%s.%d", client.GetId(), topic, partition)

	self = &Consumer{
		quit: make(chan bool),
		done: make(chan bool),
		log:  loggo.GetLogger(name),
	}

	self.master, err = sarama.NewConsumer(client.brokers, config)
	if err != nil {
		self.log.Errorf("failed to create consumer: %s", err)
		return nil, err
	}

	self.PartitionConsumer, err = self.master.ConsumePartition(topic, partition, offset)
	if err != nil {
		self.log.Errorf("failed to create consumer: %s", err)
		return nil, err
	}

	return self, nil
}

func (self *Consumer) Start(events chan *sarama.ConsumerMessage) {
	self.running = true
	for {
		select {
		case event := <-self.Messages():
			events <- event
		case <-self.quit:
			self.running = false
			close(self.done)
			return
		}
	}
}

func (self *Consumer) Shutdown() {
	if self.running {
		self.log.Infof("shutting down consumer run loop")
		close(self.quit)
		self.log.Infof("waiting for run loop to finish")
		<-self.done
	}
	self.log.Infof("closing consumer")
	self.Close()
	self.log.Infof("closing kafka client")
	self.master.Close()
	self.log.Infof("shutdown done")
}
