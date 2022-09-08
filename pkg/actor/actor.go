package actor

import (
	"context"

	"github.com/wenttang/scheduler/pkg/apis/v1alpha1"
)

type Result struct {
	Status v1alpha1.ConditionStatus `json:"status,omitempty"`

	TaskRun []*v1alpha1.TaskStatus `json:"task_run,omitempty"`

	Message string `json:"message,omitempty"`

	Reason *v1alpha1.Reason `json:"reason,omitempty"`
}

func (r *Result) Convert(status *v1alpha1.PipeplineRunStatus) {
	status.Status = r.Status
	status.Reason = r.Reason
	status.Message = r.Message
	status.TaskRun = r.TaskRun
}

type NamespacedName struct {
	Namespace string
	Name      string
}

type RegisterReq struct {
	Pipeline    *v1alpha1.Pipepline    `json:"pipeline,omitempty"`
	PipelineRun *v1alpha1.PipeplineRun `json:"pipeline_run,omitempty"`
}

type Req struct {
	Name string
}

type Actor struct {
	Id        string
	Register  func(context.Context, *RegisterReq) (*Result, error)
	GetStatus func(context.Context, *NamespacedName) (*Result, error)
}

func (a *Actor) Type() string {
	return "scheduler"
}

func (c *Actor) ID() string {
	return c.Id
}
