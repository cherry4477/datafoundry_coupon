package openshift

import (
	"errors"
	"fmt"
	//marathon "github.com/gambol99/go-marathon"
	//"github.com/pivotal-cf/brokerapi"
	"bufio"
	"bytes"
	"strings"
	"time"
	//"io"
	//"os"
	"crypto/tls"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net/http"
	neturl "net/url"
	"sync/atomic"
	//"golang.org/x/build/kubernetes"
	//"golang.org/x/oauth2"

	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	"github.com/openshift/origin/pkg/cmd/util/tokencmd"

	kapi "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/util/yaml"
	//"github.com/ghodss/yaml"
	"github.com/asiainfoLDP/datafoundry_coupon/log"
)

//func Init(dfHost, adminUser, adminPass string) {
//	theOC = createOpenshiftClient(dfHost, adminUser, adminPass)
//}

//==============================================================
//
//==============================================================

var theOC *OpenshiftClient // with admin token

var logger = log.GetLogger()

func adminClient() *OpenshiftClient {
	return theOC
}

//func AdminToken() string {
//	return theOC.BearerToken()
//}

//==============================================================
//
//==============================================================

// for general user
// the token must contains "Bearer "
func (baseOC *OpenshiftClient) NewOpenshiftClient(token string) *OpenshiftClient {
	oc := &OpenshiftClient{
		host:    baseOC.host,
		oapiUrl: baseOC.oapiUrl,
		kapiUrl: baseOC.kapiUrl,
	}

	oc.setBearerToken(token)

	return oc
}

type OpenshiftClient struct {
	name string

	host string
	//authUrl string
	oapiUrl string
	kapiUrl string

	namespace string
	username  string
	password  string
	//bearerToken string
	bearerToken atomic.Value
}

func httpsAddrMaker(addr string) string {
	if strings.HasSuffix(addr, "/") {
		addr = strings.TrimRight(addr, "/")
	}

	if !strings.HasPrefix(addr, "https://") {
		return fmt.Sprintf("https://%s", addr)
	}

	return addr
}

// for admin
func CreateOpenshiftClient(name, host, username, password string, durPhase time.Duration) *OpenshiftClient {
	host = httpsAddrMaker(host)
	oc := &OpenshiftClient{
		name: name,

		host: host,
		//authUrl: host + "/oauth/authorize?response_type=token&client_id=openshift-challenging-client",
		oapiUrl: host + "/oapi/v1",
		kapiUrl: host + "/api/v1",

		username: username,
		password: password,
	}
	oc.bearerToken.Store("")

	go oc.updateBearerToken(durPhase)

	return oc
}

func (oc *OpenshiftClient) BearerToken() string {
	//return oc.bearerToken
	return oc.bearerToken.Load().(string)
}

func (oc *OpenshiftClient) setBearerToken(token string) {
	oc.bearerToken.Store(token)
}

func (oc *OpenshiftClient) updateBearerToken(durPhase time.Duration) {
	for {
		clientConfig := &kclient.Config{}
		clientConfig.Host = oc.host
		clientConfig.Insecure = true
		//clientConfig.Version =

		logger.Info("Request Token from: %v", clientConfig.Host)

		token, err := tokencmd.RequestToken(clientConfig, nil, oc.username, oc.password)
		if err != nil {
			logger.Error("RequestToken error: ", err.Error())

			time.Sleep(15 * time.Second)
		} else {
			//clientConfig.BearerToken = token
			//oc.bearerToken = "Bearer " + token
			oc.setBearerToken("Bearer " + token)

			logger.Info("Name: %v, RequestToken token: %v", oc.name, token)

			// durPhase is to avoid mulitple OCs updating tokens at the same time
			time.Sleep(3*time.Hour + durPhase)
			durPhase = 0
		}
	}
}

func (oc *OpenshiftClient) request(method string, url string, body []byte, timeout time.Duration) (*http.Response, error) {
	//token := oc.bearerToken
	token := oc.BearerToken()
	if token == "" {
		return nil, errors.New("token is blank")
	}

	var req *http.Request
	var err error
	if len(body) == 0 {
		req, err = http.NewRequest(method, url, nil)
	} else {
		req, err = http.NewRequest(method, url, bytes.NewReader(body))
	}

	if err != nil {
		return nil, err
	}

	//for k, v := range headers {
	//	req.Header.Add(k, v)
	//}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", token)

	transCfg := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: transCfg,
		Timeout:   timeout,
	}
	return client.Do(req)
}

type WatchStatus struct {
	Info []byte
	Err  error
}

func (oc *OpenshiftClient) doWatch(url string) (<-chan WatchStatus, chan<- struct{}, error) {
	res, err := oc.request("GET", url, nil, 0)
	if err != nil {
		return nil, nil, err
	}
	//if res.Body == nil {
	//	return nil, nil, errors.New("response.body is nil")
	//}

	statuses := make(chan WatchStatus, 5)
	canceled := make(chan struct{}, 1)

	go func() {
		defer func() {
			close(statuses)
			res.Body.Close()
		}()

		reader := bufio.NewReader(res.Body)
		for {
			select {
			case <-canceled:
				return
			default:
			}

			line, err := reader.ReadBytes('\n')
			if err != nil {
				statuses <- WatchStatus{nil, err}
				return
			}

			statuses <- WatchStatus{line, nil}
		}
	}()

	return statuses, canceled, nil
}

func (oc *OpenshiftClient) OWatch(uri string) (<-chan WatchStatus, chan<- struct{}, error) {
	return oc.doWatch(oc.oapiUrl + "/watch" + uri)
}

func (oc *OpenshiftClient) KWatch(uri string) (<-chan WatchStatus, chan<- struct{}, error) {
	return oc.doWatch(oc.kapiUrl + "/watch" + uri)
}

const GeneralRequestTimeout = time.Duration(30) * time.Second

/*
func (oc *OpenshiftClient) doRequest (method, url string, body []byte) ([]byte, error) {
	res, err := oc.request(method, url, body, GeneralRequestTimeout)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	return ioutil.ReadAll(res.Body)
}

func (oc *OpenshiftClient) ORequest (method, uri string, body []byte) ([]byte, error) {
	return oc.doRequest(method, oc.oapiUrl + uri, body)
}

func (oc *OpenshiftClient) KRequest (method, uri string, body []byte) ([]byte, error) {
	return oc.doRequest(method, oc.kapiUrl + uri, body)
}
*/

type OpenshiftREST struct {
	oc  *OpenshiftClient
	Err error
}

//func NewOpenshiftREST(oc *OpenshiftClient) *OpenshiftREST {
//	return &OpenshiftREST{oc: oc}
//}

//client can't be nil now!!!
func NewOpenshiftREST(client *OpenshiftClient) *OpenshiftREST {
	//if client == nil {
	//	return &OpenshiftREST{oc: adminClient()}
	//}
	return &OpenshiftREST{oc: client}
}

func (osr *OpenshiftREST) doRequest(method, url string, bodyParams interface{}, into interface{}) *OpenshiftREST {
	if osr.Err != nil {
		return osr
	}

	var body []byte
	if bodyParams != nil {
		body, osr.Err = json.Marshal(bodyParams)
		if osr.Err != nil {
			return osr
		}
	}

	//println("11111 method = ", method, ", url = ", url)

	//res, osr.Err := oc.request(method, url, body, GeneralRequestTimeout) // non-name error
	res, err := osr.oc.request(method, url, body, GeneralRequestTimeout)
	osr.Err = err
	if osr.Err != nil {
		return osr
	}
	defer res.Body.Close()

	var data []byte
	data, osr.Err = ioutil.ReadAll(res.Body)
	if osr.Err != nil {
		return osr
	}

	//println("22222 len(data) = ", len(data), " , res.StatusCode = ", res.StatusCode)

	if res.StatusCode < 200 || res.StatusCode >= 400 {
		osr.Err = errors.New(string(data))
	} else {
		if into != nil {
			//println("into data = ", string(data), "\n")

			osr.Err = json.Unmarshal(data, into)
		}
	}

	return osr
}

func buildUriWithSelector(uri string, selector map[string]string) string {
	var buf bytes.Buffer
	for k, v := range selector {
		if buf.Len() > 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(k)
		buf.WriteByte('=')
		buf.WriteString(v)
	}

	if buf.Len() == 0 {
		return uri
	}

	values := neturl.Values{}
	values.Set("labelSelector", buf.String())

	if strings.IndexByte(uri, '?') < 0 {
		uri = uri + "?"
	}

	println("\n uri=", uri+values.Encode(), "\n")

	return uri + values.Encode()
}

// o

func (osr *OpenshiftREST) OList(uri string, selector map[string]string, into interface{}) *OpenshiftREST {

	return osr.doRequest("GET", osr.oc.oapiUrl+buildUriWithSelector(uri, selector), nil, into)
}

func (osr *OpenshiftREST) OGet(uri string, into interface{}) *OpenshiftREST {
	return osr.doRequest("GET", osr.oc.oapiUrl+uri, nil, into)
}

func (osr *OpenshiftREST) ODelete(uri string, into interface{}) *OpenshiftREST {
	return osr.doRequest("DELETE", osr.oc.oapiUrl+uri, &kapi.DeleteOptions{}, into)
}

func (osr *OpenshiftREST) OPost(uri string, body interface{}, into interface{}) *OpenshiftREST {
	return osr.doRequest("POST", osr.oc.oapiUrl+uri, body, into)
}

func (osr *OpenshiftREST) OPut(uri string, body interface{}, into interface{}) *OpenshiftREST {
	return osr.doRequest("PUT", osr.oc.oapiUrl+uri, body, into)
}

// k

func (osr *OpenshiftREST) KList(uri string, selector map[string]string, into interface{}) *OpenshiftREST {
	return osr.doRequest("GET", osr.oc.kapiUrl+buildUriWithSelector(uri, selector), nil, into)
}

func (osr *OpenshiftREST) KGet(uri string, into interface{}) *OpenshiftREST {
	return osr.doRequest("GET", osr.oc.kapiUrl+uri, nil, into)
}

func (osr *OpenshiftREST) KDelete(uri string, into interface{}) *OpenshiftREST {
	return osr.doRequest("DELETE", osr.oc.kapiUrl+uri, &kapi.DeleteOptions{}, into)
}

func (osr *OpenshiftREST) KPost(uri string, body interface{}, into interface{}) *OpenshiftREST {
	return osr.doRequest("POST", osr.oc.kapiUrl+uri, body, into)
}

func (osr *OpenshiftREST) KPut(uri string, body interface{}, into interface{}) *OpenshiftREST {
	return osr.doRequest("PUT", osr.oc.kapiUrl+uri, body, into)
}

//===============================================================
//
//===============================================================

func GetServicePortByName(service *kapi.Service, name string) *kapi.ServicePort {
	if service != nil {
		for i := range service.Spec.Ports {
			port := &service.Spec.Ports[i]
			if port.Name == name {
				return port
			}
		}
	}

	return nil
}

func GetPodPortByName(pod *kapi.Pod, name string) *kapi.ContainerPort {
	if pod != nil {
		for i := range pod.Spec.Containers {
			c := &pod.Spec.Containers[i]
			for j := range c.Ports {
				port := &c.Ports[j]
				if port.Name == name {
					return port
				}
			}
		}
	}

	return nil
}

func GetReplicationControllersByLabels(serviceBrokerNamespace string, labels map[string]string) ([]kapi.ReplicationController, error) {

	println("to list pods in", serviceBrokerNamespace)

	uri := "/namespaces/" + serviceBrokerNamespace + "/pods"

	rcs := kapi.ReplicationControllerList{}

	osr := NewOpenshiftREST(nil).KList(uri, labels, &rcs)
	if osr.Err != nil {
		return nil, osr.Err
	}

	return rcs.Items, osr.Err
}

//===============================================================
//
//===============================================================

// maybe the replace order is important, so using slice other than map would be better
/*
func Yaml2Json(yamlTemplates []byte, replaces map[string]string) ([][]byte, error) {
	var err error

	for old, rep := range replaces {
		etcdTemplateData = bytes.Replace(etcdTemplateData, []byte(old), []byte(rep), -1)
	}

	templates := bytes.Split(etcdTemplateData, []byte("---"))
	for i := range templates {
		templates[i] = bytes.TrimSpace(templates[i])
		println("\ntemplates[", i, "] = ", string(templates[i]))
	}

	return templates, err
}
*/

/*
func Yaml2Json(yamlTemplates []byte, replaces map[string]string) ([][]byte, error) {
	var err error
	decoder := yaml.NewYAMLToJSONDecoder(bytes.NewBuffer(yamlData))
	_ = decoder


	for {
		var t interface{}
		err = decoder.Decode(&t)
		m, ok := v.(map[string]interface{})
		if ok {

		}
	}
}
*/

/*
func Yaml2Json(yamlTemplates []byte, replaces map[string]string) ([][]byte, error) {
	for old, rep := range replaces {
		yamlTemplates = bytes.Replace(yamlTemplates, []byte(old), []byte(rep), -1)
	}

	jsons := [][]byte{}
	templates := bytes.Split(yamlTemplates, []byte("---"))
	for i := range templates {
		//templates[i] = bytes.TrimSpace(templates[i])
		println("\ntemplates[", i, "] = ", string(templates[i]))

		json, err := yaml.YAMLToJSON(templates[i])
		if err != nil {
			return jsons, err
		}

		jsons = append(jsons, json)
		println("\njson[", i, "] = ", string(jsons[i]))
	}

	return jsons, nil
}
*/

type YamlDecoder struct {
	decoder *yaml.YAMLToJSONDecoder
	Err     error
}

func NewYamlDecoder(yamlData []byte) *YamlDecoder {
	return &YamlDecoder{
		decoder: yaml.NewYAMLToJSONDecoder(bytes.NewBuffer(yamlData)),
	}
}

func (d *YamlDecoder) Decode(into interface{}) *YamlDecoder {
	if d.Err == nil {
		d.Err = d.decoder.Decode(into)
	}

	return d
}

func NewElevenLengthID() string {
	t := time.Now().UnixNano()
	bs := make([]byte, 8)
	for i := uint(0); i < 8; i++ {
		bs[i] = byte((t >> i) & 0xff)
	}
	return string(base64.RawURLEncoding.EncodeToString(bs))
}

var base32Encoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567")

func NewThirteenLengthID() string {
	t := time.Now().UnixNano()
	bs := make([]byte, 8)
	for i := uint(0); i < 8; i++ {
		bs[i] = byte((t >> i) & 0xff)
	}

	dest := make([]byte, 16)
	base32Encoding.Encode(dest, bs)
	return string(dest[:13])
}
