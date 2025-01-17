package integration

import (
	"fmt"
	. "github.com/onsi/gomega"
	"gopkg.in/resty.v1"
	"net/http"
	"testing"

	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/test"
)

//
//func TestConsumerGet(t *testing.T) {
//	h, client := test.RegisterIntegration(t)
//
//	account := h.NewRandAccount()
//	ctx := h.NewAuthenticatedContext(account)
//
//	// 401 using no JWT token
//	_, _, err := client.DefaultApi.ApiMaestroV1ConsumersIdGet(context.Background(), "foo").Execute()
//	Expect(err).To(HaveOccurred(), "Expected 401 but got nil error")
//
//	// GET responses per openapi spec: 200 and 404,
//	_, resp, err := client.DefaultApi.ApiMaestroV1ConsumersIdGet(ctx, "foo").Execute()
//	Expect(err).To(HaveOccurred(), "Expected 404")
//	Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
//
//	dino := h.NewConsumer(h.NewID())
//
//	consumer, resp, err := client.DefaultApi.ApiMaestroV1ConsumersIdGet(ctx, dino.ID).Execute()
//	Expect(err).NotTo(HaveOccurred())
//	Expect(resp.StatusCode).To(Equal(http.StatusOK))
//
//	Expect(*consumer.Id).To(Equal(dino.ID), "found object does not match test object")
//	Expect(*consumer.Kind).To(Equal("Consumer"))
//	Expect(*consumer.Href).To(Equal(fmt.Sprintf("/api/maestro/v1/consumers/%s", dino.ID)))
//	Expect(*consumer.CreatedAt).To(BeTemporally("~", dino.CreatedAt))
//	Expect(*consumer.UpdatedAt).To(BeTemporally("~", dino.UpdatedAt))
//}

func TestConsumerPost(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	// POST responses per openapi spec: 201, 409, 500
	c := openapi.Consumer{
		Name: openapi.PtrString("foobar"),
	}

	// 201 Created
	consumer, resp, err := client.DefaultApi.ApiMaestroV1ConsumersPost(ctx).Consumer(c).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusCreated))
	Expect(*consumer.Id).NotTo(BeEmpty(), "Expected ID assigned on creation")
	Expect(*consumer.Kind).To(Equal("Consumer"))
	Expect(*consumer.Href).To(Equal(fmt.Sprintf("/api/maestro/v1/consumers/%s", *consumer.Id)))

	// 400 bad request. posting junk json is one way to trigger 400.
	jwtToken := ctx.Value(openapi.ContextAccessToken)
	restyResp, err := resty.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", jwtToken)).
		SetBody(`{ this is invalid }`).
		Post(h.RestURL("/consumers"))

	Expect(restyResp.StatusCode()).To(Equal(http.StatusBadRequest))

	consumer, resp, err = client.DefaultApi.ApiMaestroV1ConsumersIdGet(ctx, *consumer.Id).Execute()
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
}

//
//func TestConsumerPatch(t *testing.T) {
//	h, client := test.RegisterIntegration(t)
//
//	account := h.NewRandAccount()
//	ctx := h.NewAuthenticatedContext(account)
//
//	// POST responses per openapi spec: 201, 409, 500
//
//	dino := h.NewConsumer("Brontosaurus")
//
//	// 200 OK
//	consumer, resp, err := client.DefaultApi.ApiMaestroV1ConsumersIdPatch(ctx, dino.ID).ConsumerPatchRequest(openapi.ConsumerPatchRequest{}).Execute()
//	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
//	Expect(resp.StatusCode).To(Equal(http.StatusOK))
//	Expect(*consumer.Id).To(Equal(dino.ID))
//	Expect(*consumer.CreatedAt).To(BeTemporally("~", dino.CreatedAt))
//	Expect(*consumer.Kind).To(Equal("Consumer"))
//	Expect(*consumer.Href).To(Equal(fmt.Sprintf("/api/maestro/v1/consumers/%s", *consumer.Id)))
//
//	jwtToken := ctx.Value(openapi.ContextAccessToken)
//	// 500 server error. posting junk json is one way to trigger 500.
//	restyResp, err := resty.R().
//		SetHeader("Content-Type", "application/json").
//		SetHeader("Authorization", fmt.Sprintf("Bearer %s", jwtToken)).
//		SetBody(`{ this is invalid }`).
//		Patch(h.RestURL("/consumers/foo"))
//
//	Expect(restyResp.StatusCode()).To(Equal(http.StatusBadRequest))
//}
//
//func TestConsumerPaging(t *testing.T) {
//	h, client := test.RegisterIntegration(t)
//
//	account := h.NewRandAccount()
//	ctx := h.NewAuthenticatedContext(account)
//
//	// Paging
//	_ = h.NewConsumerList("Bronto", 20)
//
//	list, _, err := client.DefaultApi.ApiMaestroV1ConsumersGet(ctx).Execute()
//	Expect(err).NotTo(HaveOccurred(), "Error getting consumer list: %v", err)
//	Expect(len(list.Items)).To(Equal(20))
//	Expect(list.Size).To(Equal(int32(20)))
//	Expect(list.Total).To(Equal(int32(20)))
//	Expect(list.Page).To(Equal(int32(1)))
//
//	list, _, err = client.DefaultApi.ApiMaestroV1ConsumersGet(ctx).Page(2).Size(5).Execute()
//	Expect(err).NotTo(HaveOccurred(), "Error getting consumer list: %v", err)
//	Expect(len(list.Items)).To(Equal(5))
//	Expect(list.Size).To(Equal(int32(5)))
//	Expect(list.Total).To(Equal(int32(20)))
//	Expect(list.Page).To(Equal(int32(2)))
//}
//
//func TestConsumerListSearch(t *testing.T) {
//	h, client := test.RegisterIntegration(t)
//
//	account := h.NewRandAccount()
//	ctx := h.NewAuthenticatedContext(account)
//
//	consumers := h.NewConsumerList("bronto", 20)
//
//	search := fmt.Sprintf("id in ('%s')", consumers[0].ID)
//	list, _, err := client.DefaultApi.ApiMaestroV1ConsumersGet(ctx).Search(search).Execute()
//	Expect(err).NotTo(HaveOccurred(), "Error getting consumer list: %v", err)
//	Expect(len(list.Items)).To(Equal(1))
//	Expect(list.Total).To(Equal(int32(20)))
//	Expect(*list.Items[0].Id).To(Equal(consumers[0].ID))
//}
