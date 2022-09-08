package actor

import (
	"context"

	dapr "github.com/dapr/go-sdk/client"
	"github.com/wenttang/scheduler/pkg/apis/v1alpha1"
)

type Status string

const (
	// Running is the task is executing.
	Running Status = "Running"
	// True is task completion.
	True Status = "True"
	// False is an accident that caused the workflow to end.
	False Status = "False"
)

type Params struct {
	Params []v1alpha1.KeyAndValue `json:"params,omitempty"`
}

type ReconcileTaskReq interface{}

type ReconcileTaskResp struct {
	Status  Status `json:"status,omitempty"`
	Message string `json:"message,omitempty"`

	OutPut []v1alpha1.KeyAndValue `json:"out_put,omitempty"`
}

type ClientStub struct {
	Id            string
	TYPE          string
	ReconcileTask func(context.Context, ReconcileTaskReq) (*ReconcileTaskResp, error)
}

func (c *ClientStub) Type() string {
	return c.TYPE
}

func (c *ClientStub) ID() string {
	return c.Id
}

func New(client dapr.Client, id, _t string) *ClientStub {
	clientStub := &ClientStub{
		Id:   id,
		TYPE: _t,
	}
	client.ImplActorClientStub(clientStub)
	return clientStub
}
