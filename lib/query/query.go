package query

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

//type Record = interface{}

type result struct {
	Done           bool
	TotalSize      int
	NextRecordsUrl string
	Records        []map[string]interface{}
}

func (r result) PublicRecords() []Record {
	res := make([]Record, len(r.Records))
	for i, rec := range r.Records {
		res[i] = r.publicRecord(rec)
	}
	return res
}

func (r result) publicRecord(rec map[string]interface{}) Record {
	pub := Record{
		Raw:    rec,
		Fields: make(map[string]interface{}, len(rec)-1),
	}
	for k, v := range rec {
		if k == "attributes" {
			attrs := v.(map[string]interface{})
			pub.Attributes.Type = attrs["type"].(string)
			pub.Attributes.Url = attrs["url"].(string)
		} else if strings.HasSuffix(k, "__r") {
			vm := v.(map[string]interface{})
			if _, isRelationList := vm["done"]; isRelationList {
				relRes := result{
					Done:           vm["done"].(bool),
					TotalSize:      int(vm["totalSize"].(float64)),
					NextRecordsUrl: vm["nextRecordsUrl"].(string),
					Records:        mapII2MapSI(vm["records"].([]interface{})),
				}
				pub.Fields[k] = relRes.PublicRecords()
			} else {
				pub.Fields[k] = r.publicRecord(vm)
			}
		} else {
			pub.Fields[k] = v
		}
	}
	return pub
}

type Record struct {
	Attributes struct {
		Type string
		Url  string
	}
	Fields map[string]interface{}
	Raw map[string]interface{}
}

type Options struct {
	apiVersion  string
	cmd         string
	tooling     bool
	instanceUrl string
	tail        string
	querystring string
	httpGet     func(string) ([]byte, error)
}

type PageCallback func(parent *Record, children []Record) bool

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
		var currResult result
		if err := json.Unmarshal(body, &currResult); err != nil {
			return err
		}
		getNextPage := cb(nil, currResult.PublicRecords())
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
	err := Query(func(parent *Record, children []Record) bool {
		records = append(records, children...)
		return true
	}, options...)
	return records, err
}

func mapII2MapSI(i2i []interface{}) []map[string]interface{} {
	res := make([]map[string]interface{}, len(i2i))
	for i, m := range i2i {
		res[i] = m.(map[string]interface{})
	}
	return res
}