package v1alpha1

import (
	"time"
)

// PipelineRef can be used to refer to a specific instance of a Pipeline.
type PipelineRef struct {
	// Name of the pipeline
	Name string `json:"name,omitempty"`
}

// PipeplineRunSpec defines the desired state of PipeplineRun
type PipeplineRunSpec struct {
	// +optional
	Params []KeyAndValue `json:"params,omitempty"`

	// +optional
	PipelineRef *PipelineRef `json:"pipelineRef,omitempty"`
}

type TaskStatusSpec struct {
	// Name is task name
	// +require
	Name string `json:"name,omitempty"`

	// +optional
	Output []KeyAndValue `json:"output,omitempty"`

	// +optional
	Items *AnyString `json:"items,omitempty"`

	// Status is the RunStatus
	// +optional
	Status *ConditionStatus `json:"status,omitempty"`

	// StartTime is the time the task is actually started.
	// +optional
	StartTime *time.Time `json:"startTime,omitempty"`

	// CompletionTime is the time the task completed.
	// +optional
	CompletionTime *time.Time `json:"completionTime,omitempty"`

	// +optional
	Message string `json:"message,omitempty"`
}

type TaskStatus struct {
	*TaskStatusSpec `json:",inline"`

	// +optional
	SubTaskStatus []TaskStatusSpec `json:"subTaskStatus,omitempty"`
}

type Reason string

const (
	SchedulerFail Reason = "scheduler"
)

// PipeplineRunStatus defines the observed state of PipeplineRun
type PipeplineRunStatus struct {
	// Status of the condition, one of True, False, Unknown.
	// +required
	Status ConditionStatus `json:"status"`

	// Reason is the cause of the Pipeline being failed.
	// +optional
	Reason *Reason `json:"reason,omitempty"`

	// +optional
	Message string `json:"message,omitempty"`

	// +optional
	// +listType=atomic
	TaskRun []*TaskStatus `json:"taskRun,omitempty"`
}

// Running return ture while status is conditionUnknown
func (p *PipeplineRunStatus) Running() bool {
	return p.Status == ConditionUnknown
}

// IsFinish return ture while status is conditionTrue or conditionFalse.
func (v *PipeplineRunStatus) IsFinish() bool {
	return v.Status == ConditionTrue || v.Status == ConditionFalse
}

func (v *PipeplineRunStatus) IsTrue() bool {
	return v.Status == ConditionTrue
}

// PipeplineRun is the Schema for the pipeplineruns API
type PipeplineRun struct {
	ObjectMeta `json:"metadata,omitempty"`

	Spec   PipeplineRunSpec   `json:"spec,omitempty"`
	Status PipeplineRunStatus `json:"status,omitempty"`
}
