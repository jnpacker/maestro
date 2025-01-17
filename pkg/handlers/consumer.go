package handlers

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/pkg/api/presenters"
	"github.com/openshift-online/maestro/pkg/errors"
	"github.com/openshift-online/maestro/pkg/services"
)

var _ RestHandler = consumerHandler{}

type consumerHandler struct {
	consumer services.ConsumerService
	generic  services.GenericService
}

func NewConsumerHandler(consumer services.ConsumerService, generic services.GenericService) *consumerHandler {
	return &consumerHandler{
		consumer: consumer,
		generic:  generic,
	}
}

func (h consumerHandler) Create(w http.ResponseWriter, r *http.Request) {
	var consumer openapi.Consumer
	cfg := &handlerConfig{
		&consumer,
		[]validate{
			validateEmpty(&consumer, "Id", "id"),
		},
		func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()
			dino := presenters.ConvertConsumer(consumer)
			dino, err := h.consumer.Create(ctx, dino)
			if err != nil {
				return nil, err
			}
			return presenters.PresentConsumer(dino), nil
		},
		handleError,
	}

	handle(w, r, cfg, http.StatusCreated)
}

func (h consumerHandler) Patch(w http.ResponseWriter, r *http.Request) {
	var patch openapi.ConsumerPatchRequest

	cfg := &handlerConfig{
		&patch,
		[]validate{},
		func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()
			id := mux.Vars(r)["id"]
			found, err := h.consumer.Get(ctx, id)
			if err != nil {
				return nil, err
			}

			//patch a field

			dino, err := h.consumer.Replace(ctx, found)
			if err != nil {
				return nil, err
			}
			return presenters.PresentConsumer(dino), nil
		},
		handleError,
	}

	handle(w, r, cfg, http.StatusOK)
}

func (h consumerHandler) List(w http.ResponseWriter, r *http.Request) {
	cfg := &handlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()

			listArgs := services.NewListArguments(r.URL.Query())
			var consumers = []api.Consumer{}
			paging, err := h.generic.List(ctx, "username", listArgs, &consumers)
			if err != nil {
				return nil, err
			}
			dinoList := openapi.ConsumerList{
				Kind:  "ConsumerList",
				Page:  int32(paging.Page),
				Size:  int32(paging.Size),
				Total: int32(paging.Total),
				Items: []openapi.Consumer{},
			}

			for _, dino := range consumers {
				converted := presenters.PresentConsumer(&dino)
				dinoList.Items = append(dinoList.Items, converted)
			}
			if listArgs.Fields != nil {
				filteredItems, err := presenters.SliceFilter(listArgs.Fields, dinoList.Items)
				if err != nil {
					return nil, err
				}
				return filteredItems, nil
			}
			return dinoList, nil
		},
	}

	handleList(w, r, cfg)
}

func (h consumerHandler) Get(w http.ResponseWriter, r *http.Request) {
	cfg := &handlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			id := mux.Vars(r)["id"]
			ctx := r.Context()
			consumer, err := h.consumer.Get(ctx, id)
			if err != nil {
				return nil, err
			}

			return presenters.PresentConsumer(consumer), nil
		},
	}

	handleGet(w, r, cfg)
}

func (h consumerHandler) Delete(w http.ResponseWriter, r *http.Request) {
	cfg := &handlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			return nil, errors.NotImplemented("delete")
		},
	}
	handleDelete(w, r, cfg, http.StatusNoContent)
}
