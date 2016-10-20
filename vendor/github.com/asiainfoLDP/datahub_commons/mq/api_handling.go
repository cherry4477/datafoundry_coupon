package mq

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/asiainfoLDP/datahub_commons/log"
)

// please make sure consumeTopic is unique and only used in current app instance
func (mq *KafukaMQ) EnableApiHandling(localServerPort int, consumeTopic string, offset int64) error {
	arl := newApiRequestListener(mq, localServerPort, consumeTopic)
	err := mq.SetMessageListener(arl.consumeTopic, arl.partition, offset, arl)
	if err != nil {
		// kafka will fail to consumer a non-existed topic, so we try to create it by send a message
		// this message will be ignored
		partition, offset2, err2 := mq.SendSyncMessage(arl.consumeTopic, []byte(""), []byte(""))
		if err2 != nil {
			log.DefaultlLogger().Warningf("EnableApiHandling error: %s", err.Error())
			return err
		}

		// try again
		arl.partition = partition
		err = mq.SetMessageListener(arl.consumeTopic, arl.partition, offset2, arl)

		if err != nil {
			log.DefaultlLogger().Warningf("EnableApiHandling error: %s", err.Error())
			return err
		}
	}

	mq.apiRequestListener = arl

	return nil
}

func (mq *KafukaMQ) DisableApiHandling() {
	if mq.apiRequestListener != nil {
		mq.SetMessageListener(mq.apiRequestListener.consumeTopic, mq.apiRequestListener.partition, 0, nil)
		mq.apiRequestListener = nil
	}
}

//=============================================================
//
//=============================================================

type ApiRequestListener struct {
	mq MessageQueue

	localServerPort int

	consumeTopic string
	partition    int32 // now always 0
}

func newApiRequestListener(mq MessageQueue, localServerPort int, consumeTopic string) *ApiRequestListener {
	return &ApiRequestListener{
		mq: mq,

		localServerPort: localServerPort,

		consumeTopic: consumeTopic,
		partition:    0,
	}
}

func (listener *ApiRequestListener) OnMessage(topic string, partition int32, offset int64, key, value []byte) bool {
	//log.DefaultlLogger().Debugf("(%d) Message consuming key: %s, value %s", offset, string(key), string(value))
	if len(key) == 0 && len(value) == 0 {
		return true // this is a message to create a topic, so it will be ignored
	}

	mq_request, err := DecodeRequest(value)
	if mq_request == nil {
		log.DefaultlLogger().Errorf("bad message api request [%d]: %s", len(value), string(value))
		return true
	}
	if err != nil {
		log.DefaultlLogger().Errorf("bad message api request [%d], error: %s\n%s\n", len(value), err.Error(), string(value))
		return true
	}

	request := mq_request.HttpRequest
	local_url := fmt.Sprintf("http://localhost:%d%s", listener.localServerPort, request.RequestURI)
	request.URL, err = url.Parse(local_url)
	if err != nil {
		log.DefaultlLogger().Errorf("local url (%s) parse error: %s", local_url, err.Error())
		return true
	}

	request.RequestURI = "" // must do this. otherwise error
	client := &http.Client{
		Timeout: time.Duration(5) * time.Second,
	}
	response, err := client.Do(request)

	if mq_request.ResponseTopic == mqp_VOID_MESSAGE_TOPIC {
		return true // async requests don't need response
	}

	var status_code int
	if err != nil {
		status_code = ResponseStatusCode_HandlingError
	} else {
		status_code = ResponseStatusCode_OK
	}

	mq_response := newMqResponse(response, "", status_code, ResponseStatusMessages[status_code])
	data, err := EncodeResponse(mq_response, mq_request.RequestID)
	if err != nil {
		log.DefaultlLogger().Errorf("EncodeResponse error: %s", err.Error())
		return true
	}

	listener.mq.SendAsyncMessage(mq_request.ResponseTopic, []byte(""), data)

	return true
}

func (listener *ApiRequestListener) OnError(err error) bool {
	log.DefaultlLogger().Debugf("api request listener error: %s", err.Error())
	return false
}
