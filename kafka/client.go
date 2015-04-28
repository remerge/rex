package kafka

import (
	"strings"

	"github.com/Shopify/sarama"
	"github.com/juju/loggo"
)

type Client struct {
	sarama.Client
	id      string
	brokers []string
	log     loggo.Logger
}

func NewClient(id string, broker_list string) (*Client, error) {
	sarama.Logger = loggerWrapper{loggo.GetLogger("sarama")}

	self := &Client{
		id:      id,
		brokers: strings.Split(broker_list, ","),
		log:     loggo.GetLogger("kafka.client." + id),
	}

	return self, nil
}

func (self *Client) GetId() string {
	return self.id
}

func (self *Client) GetGroupOffset(group string, topic string, partition int32, offset int64) (int64, error) {
	broker, err := self.Leader(topic, partition)
	if err != nil {
		return 0, err
	}

	request := &sarama.OffsetRequest{}
	request.AddBlock(topic, partition, offset, 100)
	offsets, err := broker.GetAvailableOffsets(request)
	if err != nil {
		return 0, err
	}
	block := offsets.GetBlock(topic, partition)
	return block.Offsets[0], nil
}
