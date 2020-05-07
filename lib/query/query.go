// Package query implements querying through the Salesforce data service.
// It is a non-trivial problem because:
//
// - The API response shape is weird
// - Subqueries can themselves be paginated.
//
// The first is generally manageable and nothing so out of the ordinary.
// The second however is a big problem from an API/interface perspective.
// It doesn't make sense to paginate subqueries to a caller,
// because they probably don't have the context to use the subquery.
// It's also just incredibly difficult to program the caller-based pagination of subqueries.
//
// So we choose to hide the subquery pagination as an implementation detail.
// Again, not ideal, but probably the best choice.
//
// We do, however, still support top-level query pagination.
package query

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type result struct {
	Done           bool
	TotalSize      int
	NextRecordsUrl string
	Records        []map[string]interface{}
}

func recordsFromMapRecords(q queryer, records []map[string]interface{}) ([]Record, error) {
	res := make([]Record, len(records))
	for i, rec := range records {
		rec, err := recordFromMapRecord(q, rec)
		if err != nil {
			return nil, err
		}
		res[i] = rec
	}
	return res, nil
}

func recordFromMapRecord(q queryer, rec map[string]interface{}) (Record, error) {
	res := Record{
		Raw:    rec,
		Fields: make(map[string]interface{}, len(rec)-1),
	}
	for k, v := range rec {
		if k == "attributes" {
			attrs := v.(map[string]interface{})
			res.Attributes.Type = attrs["type"].(string)
			res.Attributes.Url = attrs["url"].(string)
		} else if strings.HasSuffix(k, "__r") {
			vm := v.(map[string]interface{})
			if _, isRelationList := vm["done"]; isRelationList {
				subrecs, err := recordsFromMapRecords(q, toStrIfaceMapSlice(vm["records"].([]interface{})))
				if err != nil {
					return res, err
				}
				if !vm["done"].(bool) {
					err := q.getAllPages(vm["nextRecordsUrl"].(string), func(records []Record) bool {
						subrecs = append(subrecs, records...)
						return true
					})
					if err != nil {
						return res, err
					}
				}
				res.Fields[k] = subrecs
			} else {
				rec, err := recordFromMapRecord(q, vm)
				if err != nil {
					return res, err
				}
				res.Fields[k] = rec
			}
		} else {
			res.Fields[k] = v
		}
	}
	return res, nil
}

type Record struct {
	Attributes struct {
		Type string
		Url  string
	}
	Fields map[string]interface{}
	Raw    map[string]interface{}
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

type PageCallback func([]Record) bool

func (o Options) UrlTail() string {
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
	return fmt.Sprintf("%s%s", tail, query)
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
	q := queryer{opts.instanceUrl, opts.httpGet}
	return q.getAllPages(opts.UrlTail(), cb)
}

type queryer struct {
	instanceUrl string
	httpGet     HttpGetter
}

func (q queryer) getAllPages(nextRecordsUrl string, cb PageCallback) error {
	done := false
	for !done {
		body, err := q.httpGet(fmt.Sprintf("%s%s", q.instanceUrl, nextRecordsUrl))
		if err != nil {
			return err
		}
		currResult := result{}
		if err := json.Unmarshal(body, &currResult); err != nil {
			return err
		}
		records, err := recordsFromMapRecords(q, currResult.Records)
		if err != nil {
			return err
		}
		getNextPage := cb(records)
		if !getNextPage {
			break
		}
		done = currResult.Done
		nextRecordsUrl = currResult.NextRecordsUrl
	}
	return nil
}

func Eager(options ...Option) ([]Record, error) {
	records := make([]Record, 0, 128)
	err := Query(func(children []Record) bool {
		records = append(records, children...)
		return true
	}, options...)
	return records, err
}

func toStrIfaceMapSlice(i2i []interface{}) []map[string]interface{} {
	res := make([]map[string]interface{}, len(i2i))
	for i, m := range i2i {
		res[i] = m.(map[string]interface{})
	}
	return res
}
