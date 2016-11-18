package api

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
)

func TimeoutHandle(dt time.Duration, h httprouter.Handle) httprouter.Handle {
	return TimeoutHandleWithMessage(h, dt, "")
}

func TimeoutHandleWithMessage(h httprouter.Handle, dt time.Duration, msg string) httprouter.Handle {
	var body []byte
	if msg == "" {
		body = []byte("<html><head><title>Timeout</title></head><body><h1>Timeout</h1></body></html>")
	} else {
		body = []byte(msg)
	}

	return func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		done := make(chan bool, 1)

		tw := &timeoutWriter{w: w}
		go func() {
			h(tw, r, params)
			done <- true
		}()
		select {
		case <-done:
			return
		case <-time.After(dt):
			tw.mu.Lock()
			defer tw.mu.Unlock()
			if !tw.wroteHeader {
				tw.w.WriteHeader(http.StatusServiceUnavailable)
				tw.w.Write(body)
			}
			tw.timedOut = true
			logger.Warn("timeout: %s", r.URL.String())
		}
	}
}

var ErrHandleTimeout = errors.New("http: Handle timeout")

type timeoutWriter struct {
	w http.ResponseWriter

	mu          sync.Mutex
	timedOut    bool
	wroteHeader bool
}

func (tw *timeoutWriter) Header() http.Header {
	return tw.w.Header()
}

func (tw *timeoutWriter) Write(p []byte) (int, error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	tw.wroteHeader = true
	if tw.timedOut {
		return 0, ErrHandleTimeout
	}
	return tw.w.Write(p)
}

func (tw *timeoutWriter) WriteHeader(code int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut || tw.wroteHeader {
		return
	}
	tw.wroteHeader = true
	tw.w.WriteHeader(code)
}
