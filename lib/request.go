package lib

import (
	"io"
	"io/ioutil"
	"net/http"
)

// Request configures a Salesforce API request.
// Use Force.NewRequest to create a new request.
//
// By default, Force does not process the HTTP response body in the case of success
// (it may parse it in the case of an error). In this case, you are responsible
// for closing (and optionally reading) the response body.
//
// If you want the body to be read synchronously, use ReadResponseBody().
// You should not use Response.HttpResponse's Body in this case.
type Request struct {
	f                *Force
	method           string
	fullUrl          string
	Headers          map[string]string
	body             io.Reader
	readResponseBody bool
	callback         HttpCallback
	unauthed         bool
}

// Response is the result of a Salesforce API call.
type Response struct {
	// If Request.ReadResponseBody is used,
	// the HTTP response body is read into Response.ReadResponseBody.
	// This covers a common use case where the caller wants
	// to read the entire body synchronously,
	// rather than having to worry about managing the stream.
	ReadResponseBody []byte
	// The raw http.Response returned by the HTTP request.
	HttpResponse *http.Response
	// The coerced ContentType from the Content-Type header.
	ContentType ContentType
}

func (f *Force) NewRequest(httpMethod string) *Request {
	return &Request{f: f, method: httpMethod, Headers: map[string]string{}}
}

// RestUrl is used when the url specifies the "Apex REST" portion of the url.
// For example, the url of "/MyApexRestClass" would use a full URL of
// https://me.salesforce.com/services/data/41.0/MyApexRESTClass.
func (r *Request) RestUrl(url string) *Request {
	return r.AbsoluteUrl(r.f.fullRestUrl(url))
}

// RestUrl is used when the url specifies the root-based relative URL of a resource.
// For example, the url of "/services/async/42.0/job" would use a full URL of
// https://me.salesforce.com/services/async/42.0/job.
func (r *Request) RootUrl(url string) *Request {
	return r.AbsoluteUrl(r.f.qualifyUrl(url))
}

// AbsoluteUrl is used when the url specifies the absolute url,
// such as "https://me.salesforce.com/services/async/42.0/job".
func (r *Request) AbsoluteUrl(url string) *Request {
	r.fullUrl = url
	return r
}

// WithHeader sets the given header.
func (r *Request) WithHeader(k, v string) *Request {
	r.Headers[k] = v
	return r
}

// WithContent sets the Content-Type header.
func (r *Request) WithContent(ct ContentType) *Request {
	return r.WithHeader("Content-Type", string(ct))
}

// WithBody sets the HTTP request body.
func (r *Request) WithBody(rdr io.Reader) *Request {
	r.body = rdr
	return r
}

// ReadResponseBody specifies that the request should read and close
// the response body. Use when you want a synchronous read of the response.
// Be careful with large response bodies.
func (r *Request) ReadResponseBody() *Request {
	r.readResponseBody = true
	return r
}

// WithResponseCallback specifies a callback invoked with the *http.Response of a request.
// Most callers will not need this when invoking Request.Execute directly,
// since they have access to the *http.Response from the Response.
// However when a method does not deal with Request and Response,
// WithResponseCallback can be useful to allow access to the response,
// usually to access the HTTP response body stream.
func (r *Request) WithResponseCallback(cb HttpCallback) *Request {
	r.callback = cb
	return r
}

// Unauthed will send the request without authentication headers.
func (r *Request) Unauthed() *Request {
	r.unauthed = true
	return r
}

// Execute executes an HTTP request based on Request,
// processes the HTTP response in the configured way,
// and returns the Force Response.
//
// Execute will retry once on a SessionExpired error
// (future versions may allow configurable retry behavior).
func (r *Request) Execute() (*Response, error) {
	reqResp := &Response{}
	inp := &httpRequestInput{
		Method:  r.method,
		Url:     r.fullUrl,
		Headers: r.Headers,
		Callback: func(resp *http.Response) error {
			reqResp.HttpResponse = resp
			reqResp.ContentType = ContentType(resp.Header.Get("Content-Type"))
			if r.readResponseBody {
				b, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					return err
				}
				resp.Body.Close()
				reqResp.ReadResponseBody = b
			} else if r.callback != nil {
				return r.callback(resp)
			}
			return nil
		},
		Retrier: (&httpRetrier{}).Reauth(),
		Body:    r.body,
	}
	if !r.unauthed {
		r.f.setHttpInputAuth(inp)
	}
	err := r.f.makeHttpRequest(inp)
	return reqResp, err
}
