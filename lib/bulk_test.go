package lib_test

import (
	. "github.com/ForceCLI/force/lib"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/ghttp"
	"net/http"
)

var _ = Describe("bulk", func() {
	var sfServer *Server
	var f *Force

	BeforeEach(func() {
		sfServer = NewServer()
		f = NewForce(&ForceSession{InstanceUrl: sfServer.URL()})
	})
	AfterEach(func() {
		sfServer.Close()
	})

	jobInfo := JobInfo{ContentType: string(JobContentTypeJson)}
	Describe("RetrieveBulkJobQueryResultsWithCallback", func() {

		It("invokes the callback with the response", func() {
			sfServer.AppendHandlers(
				CombineHandlers(
					VerifyRequest("GET", "/services/async/45.0/job//batch/batch1/result/result1"),
					RespondWith(200, "abc"),
				),
			)

			f := NewForce(&ForceSession{InstanceUrl: sfServer.URL()})
			called := false
			cb := func(res *http.Response) error {
				Expect(mustRead(res.Body)).To(BeEquivalentTo("abc"))
				called = true
				return nil
			}
			Expect(f.RetrieveBulkJobQueryResultsWithCallback(jobInfo, "batch1", "result1", cb)).To(Succeed())
			Expect(called).To(BeTrue())
		})
		It("propagates errors", func() {
			sfServer.AppendHandlers(
				CombineHandlers(
					VerifyRequest("GET", "/services/async/45.0/job//batch/batch1/result/result1"),
					RespondWith(400, "whoops"),
				),
			)
			Expect(f.RetrieveBulkJobQueryResultsWithCallback(jobInfo, "batch1", "result1", nil)).To(MatchError("whoops"))
		})
	})
})
