package lib

import (
	"errors"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func newTestHttpRetrier(retryOnErrors ...func(res *http.Response, err error) bool) *HttpRetrier {
	return NewHttpRetrier(2, time.Duration(0), retryOnErrors...)

}

var _ = Describe("HttpRetrier", func() {
	It("retries at most max attempts", func() {
		calls := 0
		retrier := newTestHttpRetrier(
			func(res *http.Response, err error) bool {
				return err == nil
			},
			func(res *http.Response, err error) bool {
				calls++
				return err == err
			},
		)
		request := NewRequest("i am fundamentally wrong").RootUrl("wrong.com")
		force := &Force{Credentials: &ForceSession{InstanceUrl: "instance.com"}}

		force.WithRetrier(retrier).ExecuteRequest(request)

		retrier.shouldRetry(nil, errors.New("i am fundamentally wrong"))

		Expect(calls).To(Equal(retrier.maxAttempts))
	})

	It("retries only on specified errors", func() {
		calls := 0
		retrier := newTestHttpRetrier()
		request := NewRequest("i am fundamentally wrong").RootUrl("wrong.com")
		force := &Force{Credentials: &ForceSession{InstanceUrl: "instance.com"}}

		force.WithRetrier(retrier).ExecuteRequest(request)

		retrier.shouldRetry(nil, errors.New("i am fundamentally wrong"))

		Expect(calls).To(Equal(0))
	})
})
