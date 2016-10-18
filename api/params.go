package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"
	//"net"

	"github.com/julienschmidt/httprouter"
	//"github.com/miekg/dns"

	_ "github.com/go-sql-driver/mysql"

	"github.com/asiainfoLDP/datafoundry_coupon/common"
	//"github.com/asiainfoLDP/datahub_commons/log"
	"github.com/asiainfoLDP/datafoundry_coupon/log"

	"github.com/asiainfoLDP/datahub_commons/mq"
	"github.com/astaxie/beego/logs"
	"github.com/miekg/dns"
	"net"
	"sync"
	"sync/atomic"
	"unsafe"
)

//======================================================
//
//======================================================

const (
	Platform_Local      = "local"
	Platform_DataOS     = "dataos"
	Platform_DaoCloud   = "daocloud"
	Platform_DaoCloudUT = "daocloud_ut"

	SENDER = "datafoundry_plan"
)

var Platform = Platform_DaoCloud

var Port int
var Debug = false

//var logger = log.GetLogger()

var (
	logger = log.GetLogger()
	theMQ  unsafe.Pointer
)

//======================================================
// errors
//======================================================

const (
	StringParamType_General        = 0
	StringParamType_UrlWord        = 1
	StringParamType_UnicodeUrlWord = 2
	StringParamType_Email          = 3
)

//======================================================
//
//======================================================

var Json_ErrorBuildingJson []byte

func getJsonBuildingErrorJson() []byte {
	if Json_ErrorBuildingJson == nil {
		Json_ErrorBuildingJson = []byte(fmt.Sprintf(`{"code": %d, "msg": %s}`, ErrorJsonBuilding.code, ErrorJsonBuilding.message))
	}

	return Json_ErrorBuildingJson
}

type Result struct {
	Code uint        `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

// if data only has one item, then the item key will be ignored.
func JsonResult(w http.ResponseWriter, statusCode int, e *Error, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if e == nil {
		e = ErrorNone
	}
	result := Result{Code: e.code, Msg: e.message, Data: data}
	jsondata, err := json.Marshal(&result)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(getJsonBuildingErrorJson()))
	} else {
		w.WriteHeader(statusCode)
		w.Write(jsondata)
	}
}

type QueryListResult struct {
	Total   int64       `json:"total"`
	Results interface{} `json:"results"`
}

func NewQueryListResult(count int64, results interface{}) *QueryListResult {
	return &QueryListResult{Total: count, Results: results}
}

//======================================================
//
//======================================================

func mustBoolParam(params httprouter.Params, paramName string) (bool, *Error) {
	bool_str := params.ByName(paramName)
	if bool_str == "" {
		return false, newInvalidParameterError(fmt.Sprintf("%s can't be blank", paramName))
	}

	b, err := strconv.ParseBool(bool_str)
	if err != nil {
		return false, newInvalidParameterError(fmt.Sprintf("%s=%s", paramName, bool_str))
	}

	return b, nil
}

func mustBoolParamInQuery(r *http.Request, paramName string) (bool, *Error) {
	bool_str := r.Form.Get(paramName)
	if bool_str == "" {
		return false, newInvalidParameterError(fmt.Sprintf("%s can't be blank", paramName))
	}

	b, err := strconv.ParseBool(bool_str)
	if err != nil {
		return false, newInvalidParameterError(fmt.Sprintf("%s=%s", paramName, bool_str))
	}

	return b, nil
}

func optionalBoolParamInQuery(r *http.Request, paramName string, defaultValue bool) bool {
	bool_str := r.Form.Get(paramName)
	if bool_str == "" {
		return defaultValue
	}

	b, err := strconv.ParseBool(bool_str)
	if err != nil {
		return defaultValue
	}

	return b
}

func _mustIntParam(paramName string, int_str string) (int64, *Error) {
	if int_str == "" {
		return 0, newInvalidParameterError(fmt.Sprintf("%s can't be blank", paramName))
	}

	i, err := strconv.ParseInt(int_str, 10, 64)
	if err != nil {
		return 0, newInvalidParameterError(fmt.Sprintf("%s=%s", paramName, int_str))
	}

	return i, nil
}

func mustIntParamInQuery(r *http.Request, paramName string) (int64, *Error) {
	return _mustIntParam(paramName, r.Form.Get(paramName))
}

func mustIntParamInPath(params httprouter.Params, paramName string) (int64, *Error) {
	return _mustIntParam(paramName, params.ByName(paramName))
}

func mustIntParamInMap(m map[string]interface{}, paramName string) (int64, *Error) {
	v, ok := m[paramName]
	if ok {
		i, ok := v.(float64)
		if ok {
			return int64(i), nil
		}

		return 0, newInvalidParameterError(fmt.Sprintf("param %s is not int", paramName))
	}

	return 0, newInvalidParameterError(fmt.Sprintf("param %s is not found", paramName))
}

func _optionalIntParam(intStr string, defaultInt int64) int64 {
	if intStr == "" {
		return defaultInt
	}

	i, err := strconv.ParseInt(intStr, 10, 64)
	if err != nil {
		return defaultInt
	} else {
		return i
	}
}

func optionalIntParamInQuery(r *http.Request, paramName string, defaultInt int64) int64 {
	return _optionalIntParam(r.Form.Get(paramName), defaultInt)
}

func optionalIntParamInMap(m map[string]interface{}, paramName string, defaultValue int64) int64 {
	v, ok := m[paramName]
	if ok {
		i, ok := v.(float64)
		if ok {
			return int64(i)
		}
	}

	return defaultValue
}

func mustFloatParam(params httprouter.Params, paramName string) (float64, *Error) {
	float_str := params.ByName(paramName)
	if float_str == "" {
		return 0.0, newInvalidParameterError(fmt.Sprintf("%s can't be blank", paramName))
	}

	f, err := strconv.ParseFloat(float_str, 64)
	if err != nil {
		return 0.0, newInvalidParameterError(fmt.Sprintf("%s=%s", paramName, float_str))
	}

	return f, nil
}

func mustStringParamInPath(params httprouter.Params, paramName string, paramType int) (string, *Error) {
	str := params.ByName(paramName)
	if str == "" {
		return "", newInvalidParameterError(fmt.Sprintf("path: %s can't be blank", paramName))
	}

	if paramType == StringParamType_UrlWord {
		str2, ok := common.ValidateUrlWord(str)
		if !ok {
			return "", newInvalidParameterError(fmt.Sprintf("path: %s=%s", paramName, str))
		}
		str = str2
	} else if paramType == StringParamType_UnicodeUrlWord {
		str2, ok := common.ValidateUnicodeUrlWord(str)
		if !ok {
			return "", newInvalidParameterError(fmt.Sprintf("path: %s=%s", paramName, str))
		}
		str = str2
	} else if paramType == StringParamType_Email {
		str2, ok := common.ValidateEmail(str)
		if !ok {
			return "", newInvalidParameterError(fmt.Sprintf("path: %s=%s", paramName, str))
		}
		str = str2
	} else {
		str2, ok := common.ValidateGeneralWord(str)
		if !ok {
			return "", newInvalidParameterError(fmt.Sprintf("path: %s=%s", paramName, str))
		}
		str = str2
	}

	return str, nil
}

func mustStringParamInQuery(r *http.Request, paramName string, paramType int) (string, *Error) {
	str := r.Form.Get(paramName)
	if str == "" {
		return "", newInvalidParameterError(fmt.Sprintf("query: %s can't be blank", paramName))
	}

	if paramType == StringParamType_UrlWord {
		str2, ok := common.ValidateUrlWord(str)
		if !ok {
			return "", newInvalidParameterError(fmt.Sprintf("query: %s=%s", paramName, str))
		}
		str = str2
	}

	return str, nil
}

//======================================================
//
//======================================================

//func mustCurrentUserName(r *http.Request) (string, *Error) {
//	username, _, ok := r.BasicAuth()
//	if !ok {
//		return "", GetError(ErrorCodeAuthFailed)
//	}
//
//	return username, nil
//}

func mustCurrentUserName(r *http.Request) (string, *Error) {
	username := r.Header.Get("User")
	if username == "" {
		return "", GetError(ErrorCodeAuthFailed)
	}

	// needed?
	//username, ok = common.ValidateEmail(username)
	//if !ok {
	//	return "", newInvalidParameterError(fmt.Sprintf("user (%s) is not valid", username))
	//}

	return username, nil
}

func getCurrentUserName(r *http.Request) string {
	return r.Header.Get("User")
}

func mustRepoName(params httprouter.Params) (string, *Error) {
	repo_name, e := mustStringParamInPath(params, "repname", StringParamType_UrlWord)
	if e != nil {
		return "", e
	}

	return repo_name, nil
}

func mustRepoAndItemName(params httprouter.Params) (repo_name string, item_name string, e *Error) {
	repo_name, e = mustStringParamInPath(params, "repname", StringParamType_UrlWord)
	if e != nil {
		return
	}

	item_name, e = mustStringParamInPath(params, "itemname", StringParamType_UrlWord)
	if e != nil {
		return
	}

	return
}

func OptionalOffsetAndSize(r *http.Request, defaultSize int64, minSize int64, maxSize int64) (int64, int) {
	page := optionalIntParamInQuery(r, "page", 0)
	if page < 1 {
		page = 1
	}
	page -= 1

	if minSize < 1 {
		minSize = 1
	}
	if maxSize < 1 {
		maxSize = 1
	}
	if minSize > maxSize {
		minSize, maxSize = maxSize, minSize
	}

	size := optionalIntParamInQuery(r, "size", defaultSize)
	if size < minSize {
		size = minSize
	} else if size > maxSize {
		size = maxSize
	}

	return page * size, int(size)
}

func mustOffsetAndSize(r *http.Request, defaultSize, minSize, maxSize int) (offset int64, size int, e *Error) {
	if minSize < 1 {
		minSize = 1
	}
	if maxSize < 1 {
		maxSize = 1
	}
	if minSize > maxSize {
		minSize, maxSize = maxSize, minSize
	}

	page_size := int64(defaultSize)
	if r.Form.Get("size") != "" {
		page_size, e = mustIntParamInQuery(r, "size")
		if e != nil {
			return
		}
	}

	size = int(page_size)
	if size < minSize {
		size = minSize
	} else if size > maxSize {
		size = maxSize
	}

	// ...

	page := int64(0)
	if r.Form.Get("page") != "" {
		page, e = mustIntParamInQuery(r, "page")
		if e != nil {
			return
		}
		if page < 1 {
			page = 1
		}
		page -= 1
	}

	offset = page * page_size

	return
}

//==================================================================
//
//==================================================================

func InitMQ() {
RETRY:

	//kafkas := net.JoinHostPort(KafkaAddrPort())
	ip, port := "10.1.235.98", "9092"
	kafkas := fmt.Sprintf("%s:%s", ip, port)
	logger.Info("connectMQ, kafkas = %s", kafkas)

	messageQueue, err := mq.NewMQ([]string{kafkas}) // ex. {"192.168.1.1:9092", "192.168.1.2:9092"}
	if err != nil {
		logger.Error("connectMQ error:", err.Error())
		time.Sleep(10 * time.Second)
		goto RETRY
	}

	q := &MQ{MessageQueue: messageQueue}

	atomic.StorePointer(&theMQ, unsafe.Pointer(q))

	logger.Info("MQ inited successfully.")
}

type MQ struct {
	Mutex        sync.Mutex
	MessageQueue mq.MessageQueue
}

func getMQ() *MQ {
	return (*MQ)(atomic.LoadPointer(&theMQ))
}

func KafkaAddrPort() (string, string) {
	switch Platform {
	case Platform_DaoCloud:
		entryList := dnsExchange(os.Getenv("kafka_service_name"))

		for _, v := range entryList {
			if v.Port == "9092" {
				return v.Ip, v.Port
			}
		}
	case Platform_DataOS:
		return os.Getenv(os.Getenv("ENV_NAME_KAFKA_ADDR")), os.Getenv(os.Getenv("ENV_NAME_KAFKA_PORT"))
	case Platform_DaoCloudUT:
		fallthrough
	case Platform_Local:
		return os.Getenv("MQ_KAFKA_ADDR"), os.Getenv("MQ_KAFKA_PORT")
	}

	return "", ""
}

type dnsEntry struct {
	Ip   string
	Port string
}

func dnsExchange(srvName string) []*dnsEntry {
	fiilSrvName := fmt.Sprintf("%s.service.consul", srvName)
	agentAddr := net.JoinHostPort(consulAddrPort())
	logger.Debug("DNS query %s @ %s", fiilSrvName, agentAddr)

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(fiilSrvName), dns.TypeSRV)
	m.RecursionDesired = true

	c := &dns.Client{Net: "tcp"}
	r, _, err := c.Exchange(m, agentAddr)
	if err != nil {
		logger.Error("dns  exchange err:", err)
		return nil
	}
	if r.Rcode != dns.RcodeSuccess {
		logger.Warn("dns query err:", r.Rcode)
		return nil
	}

	/*
		entries := make([]*dnsEntry, 0, 16)
		for i := len(r.Answer) - 1; i >= 0; i-- {
			answer := r.Answer[i]
			logger.Defaultloggerger().Debugf("r.Answer[%d]=%s", i, answer.String())

			srv, ok := answer.(*dns.SRV)
			if ok {
				m.SetQuestion(dns.Fqdn(srv.Target), dns.TypeA)
				r1, _, err := c.Exchange(m, agentAddr)
				if err != nil {
					logger.DefaultLogger().Warningf("dns query error: %s", err.Error())
					continue
				}

				for j := len(r1.Answer) - 1; j >= 0; j-- {
					answer1 := r1.Answer[j]
					log.DefaultLogger().Debugf("r1.Answer[%d]=%v", i, answer1)

					a, ok := answer1.(*dns.A)
					if ok {
						a.A is the node ip instead of service ip
						entries = append(entries,  &dnsEntry{ip: a.A.String(), port: fmt.Sprintf("%d", srv.Port)})
					}
				}
			}
		}

		return entries
	*/

	if len(r.Extra) != len(r.Answer) {
		e := fmt.Sprintf("len(r.Extra)(%d) != len(r.Answer)(%d)", len(r.Extra), len(r.Answer))
		logger.Warn(e)
		return nil
	}

	num := len(r.Extra)
	entries := make([]*dnsEntry, num)
	index := 0
	for i := 0; i < num; i++ {
		a, oka := r.Extra[i].(*dns.A)
		s, oks := r.Answer[i].(*dns.SRV)
		if oka && oks {
			entries[index] = &dnsEntry{Ip: a.A.String(), Port: fmt.Sprintf("%d", s.Port)}
			index++
		}
	}

	return entries[:index]
}

func consulAddrPort() (string, string) {
	return os.Getenv("CONSUL_SERVER"), os.Getenv("CONSUL_DNS_PORT")
}

func init() {

	logs.SetAlermSendingCallback(sendAlarm)
}

type alarmEvent struct {
	Sender    string    `json:"sender"`
	Content   string    `json:"content"`
	Send_time time.Time `json:"sendTime"`
}

func sendAlarm(msg string) {
	// todo: need a buffered channel to store the alarms?
	q := getMQ()
	if q == nil {
		logger.Warn("mq is nil.")
		return
	}

	event := alarmEvent{Sender: SENDER, Content: msg, Send_time: time.Now()}

	b, err := json.Marshal(&event)
	if err != nil {
		logger.Error("Marshal err:", err)
		return
	}

	_, _, err = q.MessageQueue.SendSyncMessage("to_alarm.json", []byte(""), b)
	if err != nil {
		logger.Error("sendAlarm (to_alarm.json) error: ", err)
		return
	}
	return
}
