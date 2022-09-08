package middleware

import (
	"context"

	dapr "github.com/dapr/go-sdk/client"
	"github.com/wenttang/scheduler/pkg/apis/v1alpha1"
)

type DoReq struct {
	Pipeline    *v1alpha1.Pipepline
	PipelineRun *v1alpha1.PipeplineRun
}

type ClientStub struct {
	Id   string
	TYPE string
	Do   func(context.Context, *DoReq) error
}

func (c *ClientStub) Type() string {
	return c.TYPE
}

func (c *ClientStub) ID() string {
	return c.Id
}

func NewActor(client dapr.Client, _t string) *ClientStub {
	clientStub := &ClientStub{
		Id:   "scheduler",
		TYPE: _t,
	}
	client.ImplActorClientStub(clientStub)
	return clientStub
}
