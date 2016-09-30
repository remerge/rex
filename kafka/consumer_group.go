package kafka

import (
	"github.com/Shopify/sarama"
	"github.com/juju/loggo"
	"github.com/remerge/rex/log"
)

type ConsumerGroup struct {
	Events    chan *sarama.ConsumerMessage
	consumers []*Consumer
	log       loggo.Logger
}

func (client *Client) NewConsumerGroup(group string, topic string, offsets map[int32]int64, config *sarama.Config) (*ConsumerGroup, error) {
	self := &ConsumerGroup{
		Events:    make(chan *sarama.ConsumerMessage),
		consumers: make([]*Consumer, 0),
		log:       log.GetLogger("kafka.consumer.group." + group),
	}

	partitions, err := client.Partitions(topic)
	if err != nil {
		return nil, err
	}

	self.log.Infof("topic=%s partitions=%d partitions=%v", topic, len(partitions), partitions)
	for _, p := range partitions {
		earliest, err := client.Client.GetOffset(topic, p, sarama.OffsetOldest)
		if err != nil {
			self.Shutdown()
			return nil, err
		}

		latest, err := client.Client.GetOffset(topic, p, sarama.OffsetNewest)
		if err != nil {
			self.Shutdown()
			return nil, err
		}

		resumeFrom := offsets[p]
		if earliest == latest && earliest == 0 {
			resumeFrom = 0
		}

		if earliest > resumeFrom {
			self.log.Infof("resume from corrected from=%v to %v", resumeFrom, earliest)
			resumeFrom = earliest
		}

		if resumeFrom > latest {
			self.log.Infof("resume from corrected from=%v to %v", resumeFrom, latest)
			resumeFrom = latest
		}

		self.log.Infof("new consumer for topic=%v partition=%v earliest=%v latest=%v offset=%v",
			topic, p, earliest, latest, resumeFrom)

		consumer, err := client.NewConsumer(topic, p, resumeFrom, config)
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
