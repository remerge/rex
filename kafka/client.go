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

	client := &Client{
		id:      id,
		brokers: strings.Split(broker_list, ","),
		log:     loggo.GetLogger("kafka.client." + id),
	}

	var err error
	client.Client, err = sarama.NewClient(client.brokers, nil)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (client *Client) GetId() string {
	return client.id
}

// actually this returns the earliest or latest offsets for a topic and its partitions
// the group is ignored atm - we need to implement proper offset commits and change the fucntion alter
func (client *Client) GetGroupOffset(group string, topic string, partition int32, offset int64) (int64, error) {
	return client.GetOffset(topic, partition, offset)
}

// func (client *Client) GetOffset(topic string, partition int32, offset int64) (int64, error) {
// 	broker, err := client.Leader(topic, partition)
// 	if err != nil {
// 		return 0, err
// 	}

// 	request := &sarama.OffsetRequest{}
// 	request.AddBlock(topic, partition, offset, 100)
// 	offsets, err := broker.GetAvailableOffsets(request)
// 	if err != nil {
// 		return 0, err
// 	}
// 	block := offsets.GetBlock(topic, partition)
// 	return block.Offsets[0], nil
// }

func (client *Client) GetOffsets(topic string) (earliestMap OffsetMap, latestMap OffsetMap, err error) {
	earliestMap = make(OffsetMap)
	latestMap = make(OffsetMap)
	// TODO : optimize to get this earliest and latest with one kafka call
	partitions, err := client.Partitions(topic)
	if err != nil {
		return nil, nil, err
	}

	for _, p := range partitions {
		earliest, err := client.GetOffset(topic, p, sarama.OffsetOldest)
		if err != nil {
			return nil, nil, err
		}
		earliestMap[p] = earliest

		latest, err := client.GetOffset(topic, p, sarama.OffsetNewest)
		if err != nil {
			return nil, nil, err
		}
		latestMap[p] = latest
	}
	return earliestMap, latestMap, nil
}
