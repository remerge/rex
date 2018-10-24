package kafka

import (
	"strings"

	"github.com/Shopify/sarama"
	"github.com/remerge/rex/log"
)

type Client struct {
	sarama.Client
	brokers []string
}

func WrapSaramaLogger() {
	sarama.Logger = loggerWrapper{log.GetLogger("sarama")}
}

func NewClient(id string, broker_list string) (client *Client, err error) {
	WrapSaramaLogger()
	client = &Client{
		brokers: strings.Split(broker_list, ","),
	}

	config := sarama.NewConfig()
	config.ClientID = id

	client.Client, err = sarama.NewClient(client.brokers, config)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// actually this returns the earliest or latest offsets for a topic and its partitions
// the group is ignored atm - we need to implement proper offset commits and change the fucntion alter
func (client *Client) GetGroupOffset(group string, topic string, partition int32, offset int64) (int64, error) {
	return client.GetOffset(topic, partition, offset)
}
