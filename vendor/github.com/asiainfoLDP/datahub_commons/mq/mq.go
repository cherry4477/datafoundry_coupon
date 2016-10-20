package mq

import (
	"errors"
	"fmt"
	"sync"
	//"log"
	//"os"

	"github.com/Shopify/sarama"
	//"github.com/yaxinlx/sarama"

	//"github.com/wvanbergen/kafka/consumergroup"
	//"github.com/wvanbergen/kazoo-go"

	logger "github.com/asiainfoLDP/datahub_commons/log"
)

const (
	Offset_Newest = -1
	Offset_Oldest = -2
	Offset_Marked = -3
)

//=============================================================
//
//=============================================================

type MessageQueue interface {
	Close()
	SetMessageListener(topic string, partition int32, offset int64, consumer MassageListener) error
	SendSyncMessage(topic string, key, message []byte) (int32, int64, error)
	SendAsyncMessage(topic string, key, message []byte) error
	EnableApiCalling(consumeTopic string) error
	EnableApiHandling(localServerPort int, consumeTopic string, offset int64) error
}

type MassageListener interface {
	// return whether or not the offset will be marked in server
	OnMessage(topic string, partition int32, offset int64, key, value []byte) bool

	// return whether or not to stop listenning
	OnError(error) bool
}

//=============================================================
//
//=============================================================

func createComsumerKey(topic string, partition int32) string {
	return fmt.Sprintf("%s#%d", topic, partition)
}

type KafukaComsumer struct {
	topic     string
	partition int32

	partitionOffsetManager sarama.PartitionOffsetManager
	partitionConsumer      sarama.PartitionConsumer

	massageListener MassageListener

	closeMutex sync.Mutex
	toClose    bool
	closeChan  chan struct{}
}

func newKafukaComsumer(topic string, partition int32,
	pom sarama.PartitionOffsetManager, pc sarama.PartitionConsumer, ml MassageListener) *KafukaComsumer {
	return &KafukaComsumer{
		topic:     topic,
		partition: partition,

		partitionOffsetManager: pom,
		partitionConsumer:      pc,

		massageListener: ml,

		toClose:   false,
		closeChan: make(chan struct{}, 1),
	}
}

func (consumer *KafukaComsumer) close() {
	consumer.closeMutex.Lock()
	defer consumer.closeMutex.Unlock()

	if consumer.toClose {
		return
	}
	consumer.toClose = true

	close(consumer.closeChan)

	consumer.partitionOffsetManager.Close()
	consumer.partitionConsumer.Close()
}

func (consumer *KafukaComsumer) willClose() bool {
	return consumer.toClose
}

// todo: rename -> KafkaMQ
type KafukaMQ struct {
	client        sarama.Client
	offsetManager sarama.OffsetManager
	consumer      sarama.Consumer
	syncProducer  sarama.SyncProducer
	asyncProducer sarama.AsyncProducer

	//>>>
	//zookeeperNodes  []string
	//zookeeperChroot string
	//<<<

	consumerMapMutex    sync.Mutex
	activeConsumers     map[string]*KafukaComsumer
	apiResponseListener *ApiResponseListener

	apiRequestListener *ApiRequestListener
}

func NewMQ(brokerList []string /*, zookeepers string*/ /*, c *Config*/) (MessageQueue, error) {
	var err error

	mq := &KafukaMQ{}

	config := sarama.NewConfig()
	//if c != nil {
	//	cofnig. = c.
	//}
	mq.client, err = sarama.NewClient(brokerList, config)
	if err != nil {
		return nil, err
	}

	mq.offsetManager, err = sarama.NewOffsetManagerFromClient("", mq.client)
	if err != nil {
		return nil, err
	}

	mq.consumer, err = sarama.NewConsumerFromClient(mq.client)
	if err != nil {
		return nil, err
	}

	mq.syncProducer, err = sarama.NewSyncProducerFromClient(mq.client)
	if err != nil {
		return nil, err
	}

	mq.asyncProducer, err = sarama.NewAsyncProducerFromClient(mq.client)
	if err != nil {
		return nil, err
	}

	//>>>
	//mq.zookeeperNodes, mq.zookeeperChroot = kazoo.ParseConnectionString(zookeepers)
	//<<<

	mq.activeConsumers = make(map[string]*KafukaComsumer)

	//go mq.run()

	go func() {
		//sarama.Logger = log.New(os.Stdout, "[sarama] ", log.LstdFlags)

		for err := range mq.asyncProducer.Errors() {
			logger.DefaultLogger().Warningf("mq.syncProducer err: %s", err.Error())
		}
	}()

	return mq, nil
}

//func (mq *KafukaMq) Connected() {
//	if mq.client == nil || mq.client.
//}

//func (mq *KafukaMQ) run() {
//
//}

func (mq *KafukaMQ) Close() {
	if mq.apiResponseListener != nil {
		mq.DisableApiCalling()
	}
	if mq.apiRequestListener != nil {
		mq.DisableApiHandling()
	}

	for _, c := range mq.activeConsumers {
		mq.SetMessageListener(c.topic, c.partition, 0, nil)
	}

	if mq.consumer != nil {
		mq.consumer.Close()
	}

	if mq.syncProducer != nil {
		mq.syncProducer.Close()
	}

	if mq.asyncProducer != nil {
		mq.asyncProducer.Close()
	}

	if mq.offsetManager != nil {
		mq.offsetManager.Close()
	}

	if mq.client != nil {
		mq.client.Close()
	}
}

func (mq *KafukaMQ) runComsumer(consumer *KafukaComsumer) {
	for {
		if consumer.willClose() {
			goto END
		}
		select {
		case <-consumer.closeChan:
			goto END
		case m := <-consumer.partitionConsumer.Messages():
			if consumer.massageListener.OnMessage(m.Topic, m.Partition, m.Offset, m.Key, m.Value) {
				consumer.partitionOffsetManager.MarkOffset(m.Offset, "")
			}
		case e := <-consumer.partitionConsumer.Errors():
			if consumer.massageListener.OnError(e) {
				consumer.close()
				goto END
			}
		}
	}

END:
}

func (mq *KafukaMQ) createMessageConsumer(topic string, partition int32, offset int64, listener MassageListener) (*KafukaComsumer, error) {
	if mq.consumer == nil {
		return nil, errors.New("KafukaMQ.consumer is not inited")
	}

	channel := createComsumerKey(topic, partition)

	mq.consumerMapMutex.Lock()
	defer mq.consumerMapMutex.Unlock()

	// remove old

	oldc, ok := mq.activeConsumers[channel]
	if ok {
		oldc.close()
		delete(mq.activeConsumers, channel)
	}

	// add new

	if listener == nil {
		return nil, nil
	}

	pom, err := mq.offsetManager.ManagePartition(topic, partition)
	for err != nil {
		return nil, fmt.Errorf("ManagePartition, error: %s", err.Error())
	}

	switch offset {
	case Offset_Newest:
		offset = sarama.OffsetNewest
	case Offset_Oldest:
		offset = sarama.OffsetOldest
	case Offset_Marked:
		offset, _ = pom.NextOffset()
		if offset < 0 { // never consumed or marked
			offset = sarama.OffsetOldest
		}
	default:
		if offset < 0 {
			offset = sarama.OffsetOldest
		} else {
			// specfified offset
		}
	}

	pc, err := mq.consumer.ConsumePartition(topic, partition, offset)
	for err != nil {
		pom.Close()
		return nil, fmt.Errorf("ConsumePartition, error: %s", err.Error())
	}

	// ...

	newc := newKafukaComsumer(topic, partition, pom, pc, listener)
	mq.activeConsumers[channel] = newc
	go mq.runComsumer(newc)

	return newc, nil
}

func (mq *KafukaMQ) SetMessageListener(topic string, partition int32, offset int64, listener MassageListener) error {
	_, err := mq.createMessageConsumer(topic, partition, offset, listener)
	return err
}

func (mq *KafukaMQ) SendSyncMessage(topic string, key, message []byte) (int32, int64, error) {
	if mq.syncProducer == nil {
		return -1, -1, errors.New("mq.syncProducer == nil")
	}

	defer func() {
		recover()
	}()

	return mq.syncProducer.SendMessage(&sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.ByteEncoder(key),
		Value: sarama.ByteEncoder(message),
	})
}

func (mq *KafukaMQ) SendAsyncMessage(topic string, key, message []byte) error {
	if mq.asyncProducer == nil {
		return errors.New("mq.asyncProducer == nil")
	}

	defer func() {
		recover()
	}()

	mq.asyncProducer.Input() <- &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.ByteEncoder(key),
		Value: sarama.ByteEncoder(message),
	}

	return nil
}

//=============================================================
//
//=============================================================

// this consumer group lib doesn't work.
/*
func (mq *KafukaMQ) SetMessageListenerInGroup(groupName string, topics []string, consumer MassageListener) error {
	group_config := consumergroup.NewConfig()
	group_config.Offsets.Initial = sarama.OffsetNewest
	group_config.Offsets.ProcessingTimeout = 10 * time.Second
	group_config.Offsets.ResetOffsets = false

	cg, err := consumergroup.JoinConsumerGroup(groupName, topics, mq.zookeeperNodes, group_config)
	if err != nil {
		return err
	}

	log.Infof ("new consumer in group: %s", groupName)

	msgs := cg.Messages()
	errs := cg.Errors()
	closed := consumer.Closed()

	go func() {
		for {
			select {
			case <-closed:
				goto END
			case m := <-msgs:
				if consumer.OnMessage(m.Key, m.Value, m.Topic, m.Partition, m.Offset) ...
			case e := <-errs:
				if consumer.OnError(e) ...
			}
		}
	END:
	}()

	return nil
}
*/
