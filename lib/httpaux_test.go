package lib

import (
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
				calls += 1
				return err == err
			},
		)
		request := NewRequest("i am fundamentally wrong").RootUrl("wrong.com")
		force := (&Force{Credentials: &ForceSession{InstanceUrl: "instance.com"}}).WithRetrier(retrier)

		force.ExecuteRequest(request)

		Expect(calls).To(Equal(retrier.maxAttempts))
	})

	It("retries only on errors", func() {
		calls := 0
		retrier := newTestHttpRetrier(func(res *http.Response, err error) bool {
			calls += 1
			return err == nil
		})
		request := NewRequest("GET").RootUrl("/")
		force := (&Force{Credentials: &ForceSession{InstanceUrl: "https://google.com"}}).WithRetrier(retrier)

		force.ExecuteRequest(request)

		Expect(calls).To(Equal(0))
	})

	It("retries only on specified errors", func() {
		calls := 0
		retrier := newTestHttpRetrier(func(res *http.Response, err error) bool {
			calls += 1
			return err == nil
		})
		request := NewRequest("i am fundamentally wrong").RootUrl("wrong.com")
		force := (&Force{Credentials: &ForceSession{InstanceUrl: "instance.com"}}).WithRetrier(retrier)

		force.ExecuteRequest(request)

		Expect(calls).To(Equal(1))
	})

	It("retrier is not mutated", func() {
		retrier := newTestHttpRetrier()
		request := NewRequest("i am fundamentally wrong").RootUrl("wrong.com")
		force := (&Force{Credentials: &ForceSession{InstanceUrl: "instance.com"}}).WithRetrier(retrier)

		force.ExecuteRequest(request)

		Expect(force.retrier.attempt).To(Equal(0))
		Expect(retrier.attempt).To(Equal(0))
	})
})
