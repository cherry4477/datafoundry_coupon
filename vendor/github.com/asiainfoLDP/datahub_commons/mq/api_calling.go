package mq

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/asiainfoLDP/datahub_commons/log"
)

// please make sure consumeTopic is unique and only used in current app instance
func (mq *KafukaMQ) EnableApiCalling(consumeTopic string) error {
	apl := newApiResponseListener(consumeTopic)
	err := mq.SetMessageListener(apl.consumeTopic, apl.partition, Offset_Newest, apl)
	if err != nil {
		// kafka will fail to consumer a non-existed topic, so we try to create it by send a message
		// this message will be ignored
		partition, offset2, err2 := mq.SendSyncMessage(apl.consumeTopic, []byte(""), []byte(""))

		if err2 != nil {
			log.DefaultlLogger().Warningf("EnableApiCalling error: %s", err.Error())
			return err
		}

		// try again
		apl.partition = partition
		err = mq.SetMessageListener(apl.consumeTopic, apl.partition, offset2, apl)
		if err != nil {
			log.DefaultlLogger().Warningf("EnableApiCalling error: %s", err.Error())
			return err
		}
	}

	mq.apiResponseListener = apl

	return nil
}

func (mq *KafukaMQ) DisableApiCalling() {
	if mq.apiResponseListener != nil {
		mq.SetMessageListener(mq.apiResponseListener.consumeTopic, mq.apiResponseListener.partition, 0, nil)
		mq.apiResponseListener = nil
	}
}

// sendTopic: which topic the request will be sent to
func (mq *KafukaMQ) AsyncApiRequest(sendTopic string, key []byte, req *http.Request) error {
	listener := mq.apiResponseListener
	if listener == nil {
		return errors.New("api calling is not enabled")
	}

	data, err := EncodeRequest(listener.getNextApiRequest(req, false))
	if err != nil {
		return err
	}

	err = mq.SendAsyncMessage(sendTopic, key, data)
	if err != nil {
		return err
	}

	return nil
}

// sendTopic: which topic the request will be sent to
func (mq *KafukaMQ) SyncApiRequest(sendTopic string, key []byte, req *http.Request) (*http.Response, error) {
	listener := mq.apiResponseListener
	if listener == nil {
		return nil, errors.New("api calling is not enabled")
	}

	mq_request := listener.getNextApiRequest(req, true)
	defer listener.removePendingRequest(mq_request)

	data, err := EncodeRequest(mq_request)
	if err != nil {
		return nil, err
	}

	_, _, err = mq.SendSyncMessage(sendTopic, key, data)
	if err != nil {
		return nil, err
	}

	select {
	case <-time.After(7 * time.Second): // todo, use param instead
		log.DefaultlLogger().Warning("SyncApiRequest timeout")
		err = errors.New("SyncApiRequest timeout")
	case mq_response := <-mq_request.SyncedResponse:
		if mq_response != nil {
			if mq_response.HttpReponse != nil {
				return mq_response.HttpReponse, nil
			}

			err = fmt.Errorf("Response error: %d %s", mq_response.StatusCode, mq_response.StatusMessage)
		} else {
			err = errors.New("Response unknow error")
		}
	}

	log.DefaultlLogger().Warning("SyncApiRequest response errror: ", err.Error())

	return nil, err
}

//=============================================================
//
//=============================================================

type RequestFinder interface {
	FindRequest(id int64) *MqRequest
}

// ApiResponseListener implements both MassageListener and RequestFinder
type ApiResponseListener struct {
	consumeTopic string
	partition    int32 // now always 0

	requestMutex sync.Mutex

	nextApiRequestID   int64
	pendingApiRequests map[int64]*MqRequest // store synced request only
}

func newApiResponseListener(consumeTopic string) *ApiResponseListener {
	return &ApiResponseListener{
		consumeTopic: consumeTopic,
		partition:    0,

		nextApiRequestID:   1,
		pendingApiRequests: make(map[int64]*MqRequest),
	}
}

func (listener *ApiResponseListener) OnMessage(topic string, partition int32, offset int64, key, value []byte) bool {
	//log.DefaultlLogger().Debugf("(%d) Message consuming key: %s, value %s", offset, string(key), string(value))
	if len(key) == 0 && len(value) == 0 {
		return true // this is a message to create a topic, so it will be ignored
	}

	mq_response, mq_request, err := DecodeResponse(value, listener)
	if err != nil {
		log.DefaultlLogger().Errorf("bad message api response, error: %s", err.Error())
		return true
	}
	if mq_response == nil {
		log.DefaultlLogger().Errorf("bad message api response (mq_response == null): %s", string(value))
		return true
	}
	if mq_request == nil {
		log.DefaultlLogger().Errorf("bad message api response (mq_request == null): %s", string(value))
		return true
	}
	if mq_request.SyncedResponse == nil {
		log.DefaultlLogger().Errorf("bad message api response (mq_request.SyncedResponse == nil): %s", string(value))
		return true
	}

	//mq_response.topic = topic // must be listener.consumeTopic
	//mq_response.partition = partition // must be listener.partition
	//mq_response.offset = offset
	//mq_response.key = key

	mq_request.SyncedResponse <- mq_response

	return true
}

func (listener *ApiResponseListener) OnError(err error) bool {
	log.DefaultlLogger().Debugf("api response listener error: %s", err.Error())
	return false
}

//=============================================

func (listener *ApiResponseListener) FindRequest(id int64) *MqRequest {
	return listener.pendingApiRequests[id]
}

func (listener *ApiResponseListener) getNextApiRequest(req *http.Request, syncMode bool) *MqRequest {
	listener.requestMutex.Lock()
	defer listener.requestMutex.Unlock()

	request_id := listener.nextApiRequestID
	listener.nextApiRequestID++

	var mq_request *MqRequest

	if syncMode {
		mq_request = newMqRequest(req, "", request_id, listener.consumeTopic)
		mq_request.SyncedResponse = make(chan *MqResponse, 1)
		listener.pendingApiRequests[request_id] = mq_request
	} else {
		mq_request = newMqRequest(req, "", request_id, mqp_VOID_MESSAGE_TOPIC)
	}

	return mq_request
}

func (listener *ApiResponseListener) removePendingRequest(mqRequest *MqRequest) {
	listener.requestMutex.Lock()
	defer listener.requestMutex.Unlock()

	r, ok := listener.pendingApiRequests[mqRequest.RequestID]
	if ok {
		if mqRequest != r {
			log.DefaultlLogger().Error("removePendingRequest error: mqRequest != r")
		}
		if mqRequest.SyncedResponse == nil {
			log.DefaultlLogger().Error("removePendingRequest error: mqRequest.SyncedResponse == nil")
		}

		delete(listener.pendingApiRequests, mqRequest.RequestID)
		// close(mqRequest.SyncedResponse)
		// it doesn't need and is not a good idea to close this channal.
		// otherwiase OnMessage may write on the closed channel.
	}
}
