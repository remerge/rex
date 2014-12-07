package rex

import (
	"fmt"

	"strings"
	"time"

	"github.com/Shopify/sarama"
	"github.com/juju/loggo"
)

type BrokerTopicGroup struct {
	Brokers string
	Topic   string
	Group   string
}

type loggerWrapper struct {
	loggo.Logger
}

func (self loggerWrapper) Print(v ...interface{}) {
	self.Infof(fmt.Sprint(v...))
}

func (self loggerWrapper) Println(v ...interface{}) {
	self.Infof(fmt.Sprintln(v...))
}

func (self loggerWrapper) Printf(format string, v ...interface{}) {
	self.Infof(format, v...)
}

func NewKafkaClient(service string, brokers string, config *sarama.ClientConfig) (*sarama.Client, error) {
	log := loggo.GetLogger("rex.kafka.client[" + service + "]")
	sarama.Logger = loggerWrapper{loggo.GetLogger("sarama")}

	broker_list := strings.Split(brokers, ",")
	log.Infof("connecting to brokers=%v", broker_list)

	if config == nil {
		log.Infof("using default client config")
		config = sarama.NewClientConfig()
	}

	client, err := sarama.NewClient(service, broker_list, config)
	if err != nil {
		log.Errorf("failed to connect to kafka: %s", err)
		return nil, err
	}

	return client, nil
}

func NewKafkaProducer(client *sarama.Client, config *sarama.ProducerConfig) (*sarama.Producer, error) {
	log := loggo.GetLogger("rex.kafka.producer")

	if config == nil {
		log.Infof("using default producer config")
		config = sarama.NewProducerConfig()
		config.FlushFrequency = 1 * time.Second
		config.FlushByteCount = 1280
		config.AckSuccesses = true
	}

	if config.FlushFrequency < 10*time.Millisecond {
		log.Warningf("increasing FlushFrequency to 10ms to prevent busy looping")
		config.FlushFrequency = 10 * time.Millisecond
	}

	if config.FlushByteCount < 576 {
		log.Warningf("increasing FlushByteCount to 576 to prevent poor network utilization")
		config.FlushByteCount = 576 // minimum IPv4 MTU
	}

	producer, err := sarama.NewProducer(client, config)
	if err != nil {
		log.Errorf("failed to create producer: %s", err)
		return nil, err
	}

	return producer, nil
}

func KafkaOffset(client *sarama.Client, group string, topic string, partition int32, t sarama.OffsetTime) (int64, error) {
	broker, err := client.Leader(topic, partition)
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

type kafkaConsumer struct {
	*sarama.Consumer
	quit chan bool
	done chan bool
	log  loggo.Logger
}

func (self *kafkaConsumer) Start(events chan *sarama.ConsumerEvent) {
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

func (self *kafkaConsumer) Shutdown() {
	self.log.Debugf("shutting down consumer run loop")
	close(self.quit)
	self.log.Debugf("waiting for run loop to finish")
	<-self.done
	self.log.Debugf("closing consumer")
	self.Consumer.Close()
	self.log.Debugf("shutdown done")
}

type KafkaConsumerGroup struct {
	Events    chan *sarama.ConsumerEvent
	consumers []*kafkaConsumer
	log       loggo.Logger
}

func NewKafkaConsumerGroup(client *sarama.Client, group string, topic string, offsets map[int32]int64, config *sarama.ConsumerConfig) (*KafkaConsumerGroup, error) {
	self := &KafkaConsumerGroup{
		Events:    make(chan *sarama.ConsumerEvent),
		consumers: make([]*kafkaConsumer, 0),
		log:       loggo.GetLogger("rex.kafka.consumer.group[" + group + "]"),
	}

	partitions, err := client.Partitions(topic)
	if err != nil {
		return nil, err
	}

	for _, p := range partitions {
		earliest, err := KafkaOffset(client, group, topic, p, sarama.EarliestOffset)
		if err != nil {
			self.Shutdown()
			return nil, err
		}

		latest, err := KafkaOffset(client, group, topic, p, sarama.LatestOffsets)
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

		consumer, err := sarama.NewConsumer(client, topic, p, group, config)
		if err != nil {
			self.Shutdown()
			return nil, err
		}

		kc := &kafkaConsumer{
			Consumer: consumer,
			quit:     make(chan bool),
			done:     make(chan bool),
			log:      self.log,
		}

		self.consumers = append(self.consumers, kc)

		go kc.Start(self.Events)
	}

	return self, nil
}

func (self *KafkaConsumerGroup) Shutdown() {
	self.log.Debugf("shutting down consumer group")
	for _, consumer := range self.consumers {
		self.log.Debugf("shutting down consumer %#v", consumer)
		consumer.Shutdown()
	}
}
