/*
Copyright 2014 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package unversioned

import (
	//"encoding/json"
	"fmt"
	//"io/ioutil"
	"net"
	"net/http"
	//"net/url"
	//"os"
	//"path"
	//"reflect"
	//gruntime "runtime"
	//"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	//"k8s.io/kubernetes/pkg/api"
	//"k8s.io/kubernetes/pkg/api/latest"
	//"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"
	//"k8s.io/kubernetes/pkg/util/sets"
	//"k8s.io/kubernetes/pkg/version"
)

// Config holds the common attributes that can be passed to a Kubernetes client on
// initialization.
type Config struct {
	// Host must be a host string, a host:port pair, or a URL to the base of the API.
	Host string
	// Prefix is the sub path of the server. If not specified, the client will set
	// a default value.  Use "/" to indicate the server root should be used
	Prefix string
	// Version is the API version to talk to. Must be provided when initializing
	// a RESTClient directly. When initializing a Client, will be set with the default
	// code version.
	Version string
	// Codec specifies the encoding and decoding behavior for runtime.Objects passed
	// to a RESTClient or Client. Required when initializing a RESTClient, optional
	// when initializing a Client.
	Codec runtime.Codec

	// Server requires Basic authentication
	Username string
	Password string

	// Server requires Bearer authentication. This client will not attempt to use
	// refresh tokens for an OAuth2 flow.
	// TODO: demonstrate an OAuth2 compatible client.
	BearerToken string

	// TLSClientConfig contains settings to enable transport layer security
	TLSClientConfig

	// Server should be accessed without verifying the TLS
	// certificate. For testing only.
	Insecure bool

	// UserAgent is an optional field that specifies the caller of this request.
	UserAgent string

	// Transport may be used for custom HTTP behavior. This attribute may not
	// be specified with the TLS client certificate options. Use WrapTransport
	// for most client level operations.
	Transport http.RoundTripper
	// WrapTransport will be invoked for custom HTTP behavior after the underlying
	// transport is initialized (either the transport created from TLSClientConfig,
	// Transport, or http.DefaultTransport). The config may layer other RoundTrippers
	// on top of the returned RoundTripper.
	WrapTransport func(rt http.RoundTripper) http.RoundTripper

	// QPS indicates the maximum QPS to the master from this client.  If zero, QPS is unlimited.
	QPS float32

	// Maximum burst for throttle
	Burst int
}

type KubeletConfig struct {
	// ToDo: Add support for different kubelet instances exposing different ports
	Port        uint
	EnableHttps bool

	// TLSClientConfig contains settings to enable transport layer security
	TLSClientConfig

	// Server requires Bearer authentication
	BearerToken string

	// HTTPTimeout is used by the client to timeout http requests to Kubelet.
	HTTPTimeout time.Duration

	// Dial is a custom dialer used for the client
	Dial func(net, addr string) (net.Conn, error)
}

// TLSClientConfig contains settings to enable transport layer security
type TLSClientConfig struct {
	// Server requires TLS client certificate authentication
	CertFile string
	// Server requires TLS client certificate authentication
	KeyFile string
	// Trusted root certificates for server
	CAFile string

	// CertData holds PEM-encoded bytes (typically read from a client certificate file).
	// CertData takes precedence over CertFile
	CertData []byte
	// KeyData holds PEM-encoded bytes (typically read from a client certificate key file).
	// KeyData takes precedence over KeyFile
	KeyData []byte
	// CAData holds PEM-encoded bytes (typically read from a root certificates bundle).
	// CAData takes precedence over CAFile
	CAData []byte
}

// TransportFor returns an http.RoundTripper that will provide the authentication
// or transport level security defined by the provided Config. Will return the
// default http.DefaultTransport if no special case behavior is needed.
func TransportFor(config *Config) (http.RoundTripper, error) {
	hasCA := len(config.CAFile) > 0 || len(config.CAData) > 0
	hasCert := len(config.CertFile) > 0 || len(config.CertData) > 0

	// Set transport level security
	if config.Transport != nil && (hasCA || hasCert || config.Insecure) {
		return nil, fmt.Errorf("using a custom transport with TLS certificate options or the insecure flag is not allowed")
	}

	var (
		transport http.RoundTripper
		err       error
	)

	if config.Transport != nil {
		transport = config.Transport
	} else {
		transport, err = tlsTransportFor(config)
		if err != nil {
			return nil, err
		}
	}

	// Call wrap prior to adding debugging wrappers
	if config.WrapTransport != nil {
		transport = config.WrapTransport(transport)
	}

	switch {
	case bool(glog.V(9)):
		transport = NewDebuggingRoundTripper(transport, CurlCommand, URLTiming, ResponseHeaders)
	case bool(glog.V(8)):
		transport = NewDebuggingRoundTripper(transport, JustURL, RequestHeaders, ResponseStatus, ResponseHeaders)
	case bool(glog.V(7)):
		transport = NewDebuggingRoundTripper(transport, JustURL, RequestHeaders, ResponseStatus)
	case bool(glog.V(6)):
		transport = NewDebuggingRoundTripper(transport, URLTiming)
	}

	transport, err = HTTPWrappersForConfig(config, transport)
	if err != nil {
		return nil, err
	}

	// TODO: use the config context to wrap a transport

	return transport, nil
}

var (
	// tlsTransports stores reusable round trippers with custom TLSClientConfig options
	tlsTransports = map[string]*http.Transport{}

	// tlsTransportLock protects retrieval and storage of round trippers into the tlsTransports map
	tlsTransportLock sync.Mutex
)

// tlsTransportFor returns a http.RoundTripper for the given config, or an error
// The same RoundTripper will be returned for configs with identical TLS options
// If the config has no custom TLS options, http.DefaultTransport is returned
func tlsTransportFor(config *Config) (http.RoundTripper, error) {
	// Get a unique key for the TLS options in the config
	key, err := tlsConfigKey(config)
	if err != nil {
		return nil, err
	}

	// Ensure we only create a single transport for the given TLS options
	tlsTransportLock.Lock()
	defer tlsTransportLock.Unlock()

	// See if we already have a custom transport for this config
	if cachedTransport, ok := tlsTransports[key]; ok {
		return cachedTransport, nil
	}

	// Get the TLS options for this client config
	tlsConfig, err := TLSConfigFor(config)
	if err != nil {
		return nil, err
	}
	// The options didn't require a custom TLS config
	if tlsConfig == nil {
		return http.DefaultTransport, nil
	}

	// Cache a single transport for these options
	tlsTransports[key] = util.SetTransportDefaults(&http.Transport{
		TLSClientConfig: tlsConfig,
	})
	return tlsTransports[key], nil
}

// HTTPWrappersForConfig wraps a round tripper with any relevant layered behavior from the
// config. Exposed to allow more clients that need HTTP-like behavior but then must hijack
// the underlying connection (like WebSocket or HTTP2 clients). Pure HTTP clients should use
// the higher level TransportFor or RESTClientFor methods.
func HTTPWrappersForConfig(config *Config, rt http.RoundTripper) (http.RoundTripper, error) {
	// Set authentication wrappers
	hasBasicAuth := config.Username != "" || config.Password != ""
	if hasBasicAuth && config.BearerToken != "" {
		return nil, fmt.Errorf("username/password or bearer token may be set, but not both")
	}
	switch {
	case config.BearerToken != "":
		rt = NewBearerAuthRoundTripper(config.BearerToken, rt)
	case hasBasicAuth:
		rt = NewBasicAuthRoundTripper(config.Username, config.Password, rt)
	}
	if len(config.UserAgent) > 0 {
		rt = NewUserAgentRoundTripper(config.UserAgent, rt)
	}
	return rt, nil
}


