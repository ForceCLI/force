package lib

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"runtime"
	"time"
)

var sslKeyLogWriter *os.File
var cookieJar *cookiejar.Jar

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
	ContentTypeNone ContentType = ""
	ContentTypeJson ContentType = "application/json"
	ContentTypeXml  ContentType = "application/xml"
	ContentTypeCsv  ContentType = "text/csv"
)

type HttpMethod string

const (
	HttpMethodPost  HttpMethod = http.MethodPost
	HttpMethodPatch HttpMethod = http.MethodPatch
	HttpMethodPut   HttpMethod = http.MethodPut
)

type clientOption func(*http.Client)

func redirectPostOn302(c *http.Client) {
	// From https://stackoverflow.com/a/70510879/120731
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}

		lastReq := via[len(via)-1]
		if req.Response.StatusCode == 302 && lastReq.Method == http.MethodPost {
			req.Method = http.MethodPost

			// Get the body of the original request, set here, since req.Body will be nil if a 302 was returned
			if via[0].GetBody != nil {
				var err error
				req.Body, err = via[0].GetBody()
				if err != nil {
					return err
				}
				req.ContentLength = via[0].ContentLength
			}
		}
		return nil
	}
}

func doRequest(request *http.Request, clientOptions ...clientOption) (res *http.Response, err error) {
	client := &http.Client{}
	client.Timeout = time.Duration(Timeout) * time.Millisecond
	if cookieJar == nil {
		cookieJar, err = cookiejar.New(nil)
		if err != nil {
			return nil, fmt.Errorf("Could not initialize cookie jar: %w", err)
		}
	}
	client.Jar = cookieJar
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
	for _, option := range clientOptions {
		option(client)
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
