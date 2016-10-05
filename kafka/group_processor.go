package kafka

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/Shopify/sarama"
	"github.com/juju/loggo"
	"github.com/remerge/rex/log"
	"github.com/remerge/rex/rollbar"
)

type GroupProcessable interface {
	Msg() *sarama.ConsumerMessage
}

type LoadSaver interface {
	Load(*sarama.ConsumerMessage) (GroupProcessable, error)
	Save(GroupProcessable) error
}

type GroupProcessorConfig struct {
	Name            string
	Brokers         string
	Topic           string
	GroupGen        int
	NumChangeReader int
	NumSaveWorker   int
	ConsumerConfig  *sarama.Config
}

type GroupProcessor struct {
	Config *GroupProcessorConfig

	log    loggo.Logger
	cg     sarama.ConsumerGroup
	client *Client

	saveWorkerChannels []chan GroupProcessable

	processed chan PartitionOffset

	saveWorkerDone   *Terminator
	changeReaderDone *Terminator

	loadSaver LoadSaver
}

type PartitionOffset struct {
	Partition int32
	Offset    int64
}

func NewGroupProcessorConfig(name, brokers, topic string) *GroupProcessorConfig {
	return &GroupProcessorConfig{
		Name:            name,
		Brokers:         brokers,
		Topic:           topic,
		NumChangeReader: 4,
		NumSaveWorker:   4,
	}
}

func NewGroupProcessor(config *GroupProcessorConfig, loadSaver LoadSaver) (*GroupProcessor, error) {
	// TODO - this should be somewhere else
	WrapSaramaLogger()

	// client is just here to fetch offsets atm ...
	client, err := NewClient(config.Name+"-groupprocessor-client", config.Brokers)
	if err != nil {
		return nil, err
	}
	consumerConfig := config.ConsumerConfig
	if consumerConfig == nil {
		consumerConfig = sarama.NewConfig()
		consumerConfig.Version = sarama.V0_10_0_0 // BOOOOOOOOM
		consumerConfig.Consumer.MaxProcessingTime = 30 * time.Second
		consumerConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
	}
	consumerConfig.Group.Return.Notifications = true
	consumerConfig.ClientID = config.Name
	group := fmt.Sprintf("%s.%s.%d", config.Name, config.Topic, config.GroupGen)
	cg, err := sarama.NewConsumerGroup(strings.Split(config.Brokers, ","), group, []string{config.Topic}, consumerConfig)
	if err != nil {
		return nil, err
	}

	gp := &GroupProcessor{
		Config:           config,
		cg:               cg,
		client:           client,
		changeReaderDone: NewTerminator(),
		saveWorkerDone:   NewTerminator(),
		processed:        make(chan PartitionOffset),
		log:              log.GetLogger(config.Name + ".groupprocessor." + config.Topic),
		loadSaver:        loadSaver,
	}

	return gp, nil
}

// just log some stats
func (gp *GroupProcessor) logProgess() {
	t := time.NewTicker(1 * time.Minute)
	last := time.Now()
	defer t.Stop()

	var count, lastCount int

	offsets := make(map[int32]int64)
	for {
		select {
		case n, ok := <-gp.cg.Notifications():
			// on rebalance flush offsets
			if ok {
				gp.log.Infof("rebalanced added=%v current=%v released=%v", n.Claimed, n.Current, n.Released)
				offsets = make(map[int32]int64)
			}
		case po, ok := <-gp.processed:
			if !ok {
				break
			}
			count++
			if offsets[po.Partition] < po.Offset {
				offsets[po.Partition] = po.Offset
			}
		case _, ok := <-t.C:
			if !ok {
				break
			}

			gp.client.RefreshMetadata()

			_, latest, err := gp.client.GetOffsets(gp.Config.Topic)
			if err != nil {
				rollbar.Error(rollbar.ERR, err)
			} else {
				// just log some infos on the current processing status
				lag := make(OffsetMap)
				totalLag := int64(0)
				for p, offset := range offsets {
					lag[p] = (latest[p] - 1) - offset // as this is the next offset
					totalLag = totalLag + lag[p]
				}
				deltaT := time.Now().Sub(last)
				last = time.Now()

				deltaCount := count - lastCount
				lastCount = count

				tps := float64(deltaCount) / deltaT.Seconds()
				catchup := time.Duration(float64(totalLag)/tps) * time.Second

				gp.log.Infof("msgCount=%d latest=%v offsets=%v lag=%v total_lag=%d tps=%v eta=%v", count, latest, offsets, lag, totalLag, tps, catchup)
			}
		}
	}

}

func (gp *GroupProcessor) runChangeReader() {
	for i := 0; i < gp.Config.NumChangeReader; i++ {
		go func() {
			gp.changeReaderDone.Add(1)
			defer gp.changeReaderDone.Done()
			for {
				select {
				case msg, ok := <-gp.cg.Messages():
					if !ok {
						continue
					}
					processable, err := gp.loadSaver.Load(msg)
					if err != nil {
						continue
					}
					// id := binary.BigEndian.Uint64(cu.Id)
					id := uint64(rand.Int())
					gp.saveWorkerChannels[id%uint64(gp.Config.NumSaveWorker)] <- processable
				case <-gp.changeReaderDone.C:
					return
				}
			}
		}()
	}
}

// apply changes
func (gp *GroupProcessor) runSaveWorker() {
	gp.saveWorkerChannels = make([]chan GroupProcessable, gp.Config.NumSaveWorker)

	// we want to process the user grouped per id
	for i := 0; i < gp.Config.NumSaveWorker; i++ {
		gp.saveWorkerChannels[i] = make(chan GroupProcessable)
		go func(ch chan GroupProcessable) {
			gp.saveWorkerDone.Add(1)
			defer gp.saveWorkerDone.Done()
			for {
				select {
				case <-gp.saveWorkerDone.C:
					return
				case processable, ok := <-ch:
					if !ok {
						continue
					}
					err := gp.loadSaver.Save(processable)
					if err != nil {
						continue
					}
					// seems to be ok
					msg := processable.Msg()
					gp.cg.MarkMessage(msg, "")
					// TODO - check if this might become an issue .. as it might block
					gp.processed <- PartitionOffset{msg.Partition, msg.Offset}
				}
			}
		}(gp.saveWorkerChannels[i])
	}
}

func (gp *GroupProcessor) Run() {
	go gp.logProgess()
	gp.runSaveWorker()
	gp.runChangeReader()
}

func (gp *GroupProcessor) Close() {
	// terminate change readers
	gp.log.Infof("closing change readers")
	gp.changeReaderDone.Close(gp.Config.NumChangeReader)
	gp.log.Infof("closing save workers")
	gp.saveWorkerDone.Close(gp.Config.NumSaveWorker)
	// terminate progress logging
	gp.log.Infof("closing logging")
	close(gp.processed)
	gp.processed = nil

	gp.log.Infof("closing consumer group")
	// terminate consumer cg
	err := gp.cg.Close()
	if err != nil {
		rollbar.Error(rollbar.ERR, err)
	}
	gp.log.Infof("terminated")
}

type Terminator struct {
	sync.WaitGroup
	C chan bool
}

func NewTerminator() *Terminator {
	return &Terminator{C: make(chan bool)}
}

func (t *Terminator) Close(n int) {
	for i := 0; i < n; i++ {
		t.C <- true
	}
	t.Wait()
}
