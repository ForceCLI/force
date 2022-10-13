package lib_test

import (
	"net/http"

	. "github.com/ForceCLI/force/lib"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/ghttp"
	"github.com/rgalanakis/golangal"
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

	Describe("RetrieveBulkJobQueryResultsWithCallback", func() {
		jobInfo := JobInfo{ContentType: string(JobContentTypeJson)}
		It("invokes the callback with the response", func() {
			sfServer.AppendHandlers(
				CombineHandlers(
					VerifyRequest("GET", "/services/async/"+ApiVersionNumber()+"/job//batch/batch1/result/result1"),
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
					VerifyRequest("GET", "/services/async/"+ApiVersionNumber()+"/job//batch/batch1/result/result1"),
					RespondWith(400, "<LoginFault><exceptionCode>Yo</exceptionCode></LoginFault>", XmlHeaders),
				),
			)
			Expect(f.RetrieveBulkJobQueryResultsWithCallback(jobInfo, "batch1", "result1", nil)).To(MatchError("Yo: "))
		})
	})

	Describe("GetBatches", func() {
		It("handles empty batches", func() {
			body := `<?xml version="1.0" encoding="UTF-8"?>
<batchInfoList xmlns="http://www.force.com/2009/06/asyncapi/dataload" />`
			sfServer.AppendHandlers(
				CombineHandlers(
					VerifyRequest("GET", "/services/async/"+ApiVersionNumber()+"/job/myjobid/batch"),
					RespondWith(200, body, XmlHeaders),
				),
			)
			res, err := f.GetBatches("myjobid")
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(BeEmpty())
		})
		It("handles having batches batches", func() {
			body := `<?xml version="1.0" encoding="UTF-8"?>
<batchInfoList xmlns="http://www.force.com/2009/06/asyncapi/dataload">
<batchInfo><id>batch1</id></batchInfo>
<batchInfo><id>batch2</id></batchInfo>
</batchInfoList>`
			sfServer.AppendHandlers(
				CombineHandlers(
					VerifyRequest("GET", "/services/async/"+ApiVersionNumber()+"/job/myjobid/batch"),
					RespondWith(200, body, XmlHeaders),
				),
			)
			res, err := f.GetBatches("myjobid")
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(ConsistOf(
				golangal.MatchField("Id", "batch1"),
				golangal.MatchField("Id", "batch2"),
			))
		})
		It("handles faults", func() {
			sfServer.AppendHandlers(
				CombineHandlers(
					VerifyRequest("GET", "/services/async/"+ApiVersionNumber()+"/job/myjobid/batch"),
					RespondWith(400, loginFaultBody, XmlHeaders),
				),
			)
			_, err := f.GetBatches("myjobid")
			Expect(err).To(MatchError("somecode: msg"))
		})
	})
})
