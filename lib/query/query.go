package query

import (
	"encoding/json"
	"fmt"
	"net/url"
)

type Record = interface{}

type Result struct {
	Done           bool
	TotalSize      int
	NextRecordsUrl string
	Records        []Record
}

type Options struct {
	apiVersion string
	cmd string
	tooling bool
	instanceUrl string
	tail string
	querystring string
	httpGet func(string) ([]byte, error)
}

type PageCallback func(parent Record, children []Record) bool

func (o Options) Url() string {
	tail := o.tail
	if tail == "" {
		cmd := o.cmd
		if o.tooling {
			cmd = "tooling/" + cmd
		}
		tail = fmt.Sprintf("/services/data/%s/%s", o.apiVersion, cmd)
	}
	query := ""
	if o.querystring != "" {
		query = "?q=" + url.QueryEscape(o.querystring)
	}
	return fmt.Sprintf("%s%s%s", o.instanceUrl, tail, query)
}

type Option func(*Options)

func All(o *Options) {
	o.cmd = "queryAll"
}

func Tooling(o *Options) {
	o.tooling = true
}

func InstanceUrl(url string) Option {
	return func(o *Options) {
		o.instanceUrl = url
	}
}

func Tail(tail string) Option {
	return func(o *Options) {
		o.tail = tail
	}
}

func ApiVersion(v string) Option {
	return func(o *Options) {
		o.apiVersion = v
	}
}

func QS(v string) Option {
	return func(o *Options) {
		o.querystring = v
	}
}

type HttpGetter func(string) ([]byte, error)

func HttpGet(f HttpGetter) Option {
	return func(o *Options) {
		o.httpGet = f
	}
}

func Query(cb PageCallback, options ...Option) error {
	opts := Options{cmd: "query"}
	for _, option := range options {
		option(&opts)
	}

	done := false
	nextRecordsUrl := opts.Url()
	for !done {
		body, err := opts.httpGet(nextRecordsUrl)
		if err != nil {
			return err
		}
		var currResult Result
		if err := json.Unmarshal(body, &currResult); err != nil {
			return err
		}
		getNextPage := cb(nil, currResult.Records)
		if !getNextPage {
			break
		}
		done = currResult.Done
		nextRecordsUrl = fmt.Sprintf("%s%s", opts.instanceUrl, currResult.NextRecordsUrl)
	}
	return nil
}

func Eager(options ...Option) ([]Record, error) {
	records := make([]Record, 0, 128)
	err := Query(func(parent Record, children []Record) bool {
		records = append(records, children...)
		return true
	}, options...)
	return records, err
}