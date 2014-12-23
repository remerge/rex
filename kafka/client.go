package kafka

import (
	"strings"
	"time"

	"github.com/Shopify/sarama"
	"github.com/juju/loggo"
)

type Client struct {
	*sarama.Client
	id  string
	log loggo.Logger
}

func NewClient(id string, brokers string, config *sarama.ClientConfig) (*Client, error) {
	sarama.Logger = loggerWrapper{loggo.GetLogger("sarama")}

	self := &Client{
		id:  id,
		log: loggo.GetLogger("kafka.client." + id),
	}

	broker_list := strings.Split(brokers, ",")
	self.log.Infof("connecting to brokers=%v", broker_list)

	if config == nil {
		self.log.Infof("using default client config")
		config = sarama.NewClientConfig()
		config.DefaultBrokerConf = &sarama.BrokerConfig{
			MaxOpenRequests: 4,
			DialTimeout:     5 * time.Second,
			ReadTimeout:     5 * time.Second,
			WriteTimeout:    5 * time.Second,
		}
	}

	client, err := sarama.NewClient(id, broker_list, config)
	if err != nil {
		self.log.Errorf("failed to connect to kafka: %s", err)
		return nil, err
	}

	self.log.Infof("connected")
	self.Client = client
	return self, nil
}

func (self *Client) GetId() string {
	return self.id
}

func (self *Client) GetGroupOffset(group string, topic string, partition int32, t sarama.OffsetTime) (int64, error) {
	broker, err := self.Leader(topic, partition)
	if err != nil {
		return 0, err
	}
	request := &sarama.OffsetRequest{}
	request.AddBlock(topic, partition, t, 100)
	offsets, err := broker.GetAvailableOffsets(group, request)
	if err != nil {
		return 0, err
	}
	block := offsets.GetBlock(topic, partition)
	return block.Offsets[0], nil
}
