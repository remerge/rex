package kafka

import (
	"time"

	"github.com/Shopify/sarama"
	"github.com/juju/loggo"
)

type ConsumerGroup struct {
	Events    chan *sarama.ConsumerEvent
	consumers []*Consumer
	log       loggo.Logger
}

func (client *Client) NewConsumerGroup(group string, topic string, offsets map[int32]int64, config *sarama.ConsumerConfig) (*ConsumerGroup, error) {
	self := &ConsumerGroup{
		Events:    make(chan *sarama.ConsumerEvent),
		consumers: make([]*Consumer, 0),
		log:       loggo.GetLogger("kafka.consumer.group." + group),
	}

	partitions, err := client.Partitions(topic)
	if err != nil {
		return nil, err
	}

	for _, p := range partitions {
		earliest, err := client.GetGroupOffset(group, topic, p, sarama.EarliestOffset)
		if err != nil {
			self.Shutdown()
			return nil, err
		}

		latest, err := client.GetGroupOffset(group, topic, p, sarama.LatestOffsets)
		if err != nil {
			self.Shutdown()
			return nil, err
		}

		resumeFrom := offsets[p] + 1
		if earliest == latest && earliest == 0 {
			resumeFrom = 0
		}

		if earliest > resumeFrom {
			resumeFrom = earliest
		}

		if resumeFrom > latest {
			resumeFrom = latest
		}

		if config == nil {
			config = sarama.NewConsumerConfig()
		}

		config.OffsetMethod = sarama.OffsetMethodManual
		config.OffsetValue = resumeFrom
		config.MaxWaitTime = 1 * time.Second

		self.log.Infof("new consumer for topic=%v partition=%v earliest=%v latest=%v offset=%v",
			topic, p, earliest, latest, resumeFrom)

		consumer, err := client.NewConsumer(group, topic, p, config)
		if err != nil {
			self.Shutdown()
			return nil, err
		}

		self.consumers = append(self.consumers, consumer)
		go consumer.Start(self.Events)
	}

	return self, nil
}

func (self *ConsumerGroup) Shutdown() {
	self.log.Infof("shutting down consumer group")
	for _, consumer := range self.consumers {
		self.log.Infof("shutting down consumer %#v", consumer)
		consumer.Shutdown()
	}
}
