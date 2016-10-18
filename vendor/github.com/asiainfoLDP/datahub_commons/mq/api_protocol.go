package mq

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	//"github.com/asiainfoLDP/datahub_commons/log"
)

const mqp_VOID_MESSAGE_TOPIC = "-"

const mqp_VERSION_STRING = "MQP/0.1"

var mqp_REQUEST_HEADER = "%s %d %s\n\n" // protocol_version, request_id, response_topic

var mqp_RESPONSE_HEADER = "%s %d %d %s\n\n" // protocol_version, request_id, errno, errmsg

const (
	ResponseStatusCode_OK             = 0
	ResponseStatusCode_HandlingError  = 1
	ResponseStatusCode_OutputingError = 2
	NumResponseStatusCodes            = 10
)

var ResponseStatusMessages [NumResponseStatusCodes]string

func init() {
	ResponseStatusMessages[ResponseStatusCode_OK] = "OK"
	ResponseStatusMessages[ResponseStatusCode_HandlingError] = "Handling Error"
	ResponseStatusMessages[ResponseStatusCode_OutputingError] = "Outputing Error"
}

type MqRequest struct {
	Proto         string
	RequestID     int64
	ResponseTopic string

	HttpRequest *http.Request

	SyncedResponse chan *MqResponse // for sync request only
}

func newMqRequest(req *http.Request, proto string, requestID int64, consumeTopic string) *MqRequest {
	if proto == "" {
		proto = mqp_VERSION_STRING
	}
	return &MqRequest{
		Proto:         proto,
		RequestID:     requestID,
		ResponseTopic: consumeTopic,

		HttpRequest: req,
	}
}

// consumeTopic can't contains break line
func EncodeRequest(mqRquest *MqRequest) ([]byte, error) {
	buf := new(bytes.Buffer)

	_, err := fmt.Fprintf(buf, mqp_REQUEST_HEADER, mqRquest.Proto, mqRquest.RequestID, mqRquest.ResponseTopic)
	if err != nil {
		return nil, err
	}

	// ...

	if err = mqRquest.HttpRequest.Write(buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func DecodeRequest(data []byte) (*MqRequest, error) {
	if data == nil {
		return nil, errors.New("can't decode request from nil data")
	}

	buf := bytes.NewBuffer(data)

	words, err := splitWordsInNextLineBySpace(buf, 3)
	if err != nil {
		return nil, err
	}
	if len(words) < 3 {
		return nil, fmt.Errorf("invalid mq request header: %s", words)
	}
	topic := words[2]
	if topic == "" {
		return nil, errors.New("response topic is not specified")
	}
	request_id, err := strconv.ParseInt(words[1], 10, 64)
	if err != nil {
		return nil, err
	}
	proto := words[0]

	var req *http.Request = nil
	//var err error

	for {
		if proto == mqp_VERSION_STRING {
			_, err = readNextLine(buf) // skip one line
			if err != nil {
				break
			}

			req, err = http.ReadRequest(bufio.NewReader(buf))
			break
		}

		err = errors.New("unknown mq request proto")
		break
	}

	return newMqRequest(req, proto, request_id, topic), err
}

type MqResponse struct {
	//topic     string
	//partition int32
	//offset    int64
	//key       string
	
	Proto         string
	StatusCode    int
	StatusMessage string

	HttpReponse *http.Response
}

func newMqResponse(res *http.Response, proto string, statusCode int, statusMessage string) *MqResponse {
	if proto == "" {
		proto = mqp_VERSION_STRING
	}
	return &MqResponse{
		Proto:         proto,
		StatusCode:    statusCode,
		StatusMessage: statusMessage,

		HttpReponse: res,
	}
}

// this function only return non-nil error when it is impossible to encode the header.
// mqResponse.HttpReponse.Body must not closed before calling this function
func EncodeResponse(mqResponse *MqResponse, requestId int64) ([]byte, error) {
	buf := new(bytes.Buffer)

	_, err := fmt.Fprintf(buf, mqp_RESPONSE_HEADER, mqResponse.Proto, requestId, mqResponse.StatusCode, mqResponse.StatusMessage)
	if err != nil {
		return nil, err
	}

	// ...

	for mqResponse.StatusCode == 0 && mqResponse.HttpReponse != nil {
		defer mqResponse.HttpReponse.Body.Close()

		_, err = fmt.Fprintf(buf, "%s %s", mqResponse.HttpReponse.Proto, mqResponse.HttpReponse.Status)
		if err != nil {
			break
		}

		err = mqResponse.HttpReponse.Header.Write(buf)
		if err != nil {
			break
		}

		err = buf.WriteByte('\n')
		if err != nil {
			break
		}

		_, err = io.Copy(buf, mqResponse.HttpReponse.Body)
		if err != nil {
			break
		}

		break
	}

	if err != nil {
		buf = new(bytes.Buffer)

		_, err := fmt.Fprintf(buf, mqp_RESPONSE_HEADER, mqResponse.Proto, requestId, ResponseStatusCode_OutputingError, ResponseStatusMessages[ResponseStatusCode_OutputingError])
		if err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func DecodeResponse(data []byte, requestFinder RequestFinder) (*MqResponse, *MqRequest, error) {
	if data == nil {
		return nil, nil, errors.New("can't decode response from nil data")
	}

	buf := bytes.NewBuffer(data)

	words, err := splitWordsInNextLineBySpace(buf, 4)
	if err != nil {
		return nil, nil, err
	}
	if len(words) < 4 {
		return nil, nil, fmt.Errorf("invalid mq response header: %s", words)
	}
	request_id, err := strconv.ParseInt(words[1], 10, 64)
	if err != nil {
		return nil, nil, err
	}
	req := requestFinder.FindRequest(request_id)
	if req == nil {
		return nil, req, fmt.Errorf("can't find request with id %d", request_id)
	}
	status_code, err := strconv.ParseInt(words[2], 10, 32)
	if err != nil {
		return nil, req, err
	}
	status_message := words[3]
	proto := words[0]

	var res *http.Response = nil
	//var err error

	for {
		if proto == mqp_VERSION_STRING {
			_, err = readNextLine(buf) // skip one line
			if err != nil {
				break
			}

			res, err = http.ReadResponse(bufio.NewReader(buf), req.HttpRequest)
			break
		}

		err = errors.New("unknown mq response proto")
		break
	}

	return newMqResponse(res, proto, int(status_code), status_message), req, err
}

/*
type ResponseResult struct {
	StatusCode   uint        `json:"-"`
	ErrorID      uint        `json:"code"`
	ErrorMessage string      `json:"msg"`
	Data         interface{} `json:"data,omitempty"`
}

func EncodeResponseResult (result *ResponseResult) ([]byte, error) {
	buf := new (bytes.Buffer)

	header := fmt.Sprintf ("0000\n%d\n%d\n%s\n%d\n", result.StatusCode, result.ErrorID, result.ErrorMessage, len(result.Data))
	_, err := buf.WriteString(header)
	if err != nil {
		return nil, err
	}
	if result.Data != nil {
		_, err := buf.Write(result.Data)
		if err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func DecodeResponseResult(data []byte, req *http.Request) (*ResponseResult, error) {
	buf := bytes.NewBuffer(data)

	version, err := parseTextLineAsUint(buf)
	if err != nil {
		return nil, err
	}
	status_code, err := parseTextLineAsUint(buf)
	if err != nil {
		return nil, err
	}
	}
	err_id, err := parseTextLineAsUint(buf)
	if err != nil {
		return nil, err
	}
	err_msg, err := buf.ReadString('\n')
	if len(err_msg) > 0 {
		err_msg = status_code_str[:len(err_msg)-1]
	}
	data_len, err := parseTextLineAsUint(buf)
	if err != nil {
		return nil, err
	}
	data := buf.Bytes()
	if len(data) < data_len {
		return nil, errors.New
	}

	return &ResponseResult{
			StatusCode: ,
			ErrorID: ,
			ErrorMessage: ,
			Data: ,
		}, nil
}
*/

//=============================================================
//
//=============================================================

func readNextLine(buf *bytes.Buffer) (string, error) {
	str, err := buf.ReadString('\n')
	if err != nil {
		return "", err
	}
	if len(str) > 0 {
		str = str[:len(str)-1]
	}
	return str, nil
}

func parseKeyValueInLine(line string) (string, string) {
	return parseKeyValueInLineBySep(line, ':')
}

func parseKeyValueInNextLine(buf *bytes.Buffer) (string, string, error) {
	line, err := readNextLine(buf)
	if err != nil {
		return "", "", err
	}
	k, v := parseKeyValueInLine(line)
	return k, v, nil
}

func parseKeyValueInLineBySep(line string, seq byte) (string, string) {
	index := strings.IndexByte(line, seq)
	if index == -1 {
		return "", strings.TrimSpace(line)
	}
	return strings.TrimSpace(line[:index]), strings.TrimSpace(line[index+1:])
}

func splitWordsInLineBySpace(line string, maxNum int) []string {
	words := make([]string, maxNum)
	num := 0

	for num < maxNum {
		k, v := parseKeyValueInLineBySep(line, ' ')
		if v == "" {
			break
		} else if k == "" {
			words[num] = v
			num++
			break
		} else {
			words[num] = k
			num++
			line = v
		}
	}

	return words[:num]
}

func splitWordsInNextLineBySpace(buf *bytes.Buffer, maxNum int) ([]string, error) {
	line, err := readNextLine(buf)
	if err != nil {
		return []string{}, err
	}
	return splitWordsInLineBySpace(line, maxNum), nil
}
