package lib

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"time"
)

var sslKeyLogWriter *os.File

func init() {
	if f := os.Getenv("SSLKEYLOGFILE"); f != "" {
		var err error
		sslKeyLogWriter, err = os.OpenFile(f, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			panic("Could not open SSLKEYLOGFILE: " + err.Error())
		}
	}
}

type ContentType string

const (
	ContentTypeNone = ""
	ContentTypeJson = "application/json"
	ContentTypeXml  = "application/xml"
	ContentTypeCsv  = "application/csv"
)

func doRequest(request *http.Request) (res *http.Response, err error) {
	client := &http.Client{}
	client.Timeout = time.Duration(Timeout) * time.Millisecond
	if sslKeyLogWriter != nil {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				KeyLogWriter: sslKeyLogWriter,
			},
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       10 * time.Minute,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
	}
	return client.Do(request)
}

func httpRequest(method, url string, body io.Reader) (request *http.Request, err error) {
	return httpRequestWithHeaders(method, url, nil, body)
}

func httpRequestWithHeaders(method, url string, headers map[string]string, body io.Reader) (request *http.Request, err error) {
	request, err = http.NewRequest(method, url, body)
	if err != nil {
		return
	}
	request.Header.Add("User-Agent", fmt.Sprintf("force/%s (%s-%s)", Version, runtime.GOOS, runtime.GOARCH))
	for k, v := range headers {
		request.Header.Set(k, v)
	}
	return
}

type httpRequestInput struct {
	Method   string
	Url      string
	Headers  map[string]string
	Callback HttpCallback
	Retrier  *httpRetrier
	Body     io.Reader
}

func (r *httpRequestInput) WithCallback(cb HttpCallback) *httpRequestInput {
	r.Callback = cb
	return r
}

func (r *httpRequestInput) WithHeader(k, v string) *httpRequestInput {
	r.Headers[k] = v
	return r
}

func (r *httpRequestInput) WithContent(ct ContentType) *httpRequestInput {
	return r.WithHeader("Content-Type", string(ct))
}

// HttpCallback is called after a successful HTTP request.
// The caller is responsible for closing the response body when it's finished.
type HttpCallback func(*http.Response) error

type httpRetrier struct {
	attempt       int
	maxAttempts   int
	retryOnErrors []error
}

func (r *httpRetrier) Reauth() *httpRetrier {
	if r.maxAttempts == 0 {
		r.maxAttempts = 1
	}
	r.retryOnErrors = append(r.retryOnErrors, SessionExpiredError)
	return r
}

func (r *httpRetrier) Attempts(max int) *httpRetrier {
	r.maxAttempts = max
	return r
}

func (r *httpRetrier) ShouldRetry(res *http.Response, err error) bool {
	if err == nil {
		return false
	}
	if r.attempt >= r.maxAttempts {
		return false
	}
	r.attempt += 1
	for _, e := range r.retryOnErrors {
		if err == e {
			return true
		}
	}
	return false
}
