package actor

import (
	"context"

	dapr "github.com/dapr/go-sdk/client"
	"github.com/wenttang/workflow/pkg/apis/v1alpha1"
)

type Status string

const (
	Running Status = "Running"
	True    Status = "True"
	False   Status = "False"
)

type ReconcileTaskReq struct {
	Params []v1alpha1.KeyAndValue
}
type ReconcileTaskResp struct {
	Status  Status
	Message string

	OutPut []v1alpha1.KeyAndValue
}

type ClientStub struct {
	Id            string
	TYPE          string
	ReconcileTask func(context.Context, *ReconcileTaskReq) (*ReconcileTaskResp, error)
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
