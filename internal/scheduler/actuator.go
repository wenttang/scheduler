package scheduler

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	schedulerActor "github.com/wenttang/scheduler/pkg/actor"
	"github.com/wenttang/workflow/pkg/apis/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Actuator struct {
	*ActorSet

	Name        string
	Pipeline    *v1alpha1.Pipepline    `json:"pipeline,omitempty"`
	PipelineRun *v1alpha1.PipeplineRun `json:"pipeline_run,omitempty"`

	logger log.Logger
}

func (a *Actuator) Reconcile(ctx context.Context) error {
	task := a.getTask()
	if task == nil {
		a.PipelineRun.Status.Status = corev1.ConditionTrue
		return nil
	}

	if !task.isStarted() {
		now := metav1.Now()
		task.taskRun.StartTime = &now
		err := task.parseParams(a.PipelineRun.Spec.Params)
		if err != nil {
			state := corev1.ConditionFalse
			task.taskRun.Status = &state
			return err
		}
		state := corev1.ConditionUnknown
		task.taskRun.Status = &state
	}

	actor := a.getActor(task.task.Actor.Type)

	resp, err := actor.ReconcileTask(ctx, &schedulerActor.ReconcileTaskReq{
		Params: task.task.Params,
	})
	if err != nil {
		level.Error(a.logger).Log("message", err.Error())
		return err
	}

	var state corev1.ConditionStatus
	switch resp.Status {
	case schedulerActor.Running:
		state = corev1.ConditionUnknown
	case schedulerActor.True:
		state = corev1.ConditionTrue
		now := metav1.Now()
		task.taskRun.CompletionTime = &now
	default:
		state = corev1.ConditionFalse
	}

	task.taskRun.Status = &state
	task.taskRun.Output = resp.OutPut
	return nil
}

func (a *Actuator) ParseParams() error {
	var getParams = func(params []v1alpha1.KeyAndValue, name string) *v1alpha1.KeyAndValue {
		for _, param := range params {
			if param.Name == name {
				return &param
			}
		}
		return nil
	}

	result := make([]v1alpha1.KeyAndValue, 0, len(a.Pipeline.Spec.Params))
	for _, paramSpec := range a.Pipeline.Spec.Params {
		param := getParams(a.PipelineRun.Spec.Params, paramSpec.Name)
		if param == nil {
			if paramSpec.Default == nil {
				return fmt.Errorf("%s is required", param.Name)
			}
			param = &v1alpha1.KeyAndValue{
				Name:  paramSpec.Name,
				Value: paramSpec.Default,
			}
		}
		result = append(result, *param)
	}

	a.PipelineRun.Spec.Params = result
	return nil
}

func (a *Actuator) getTask() *task {
	l := len(a.PipelineRun.Status.TaskRun)
	if l == 0 {
		state := corev1.ConditionUnknown
		a.PipelineRun.Status.TaskRun = []*v1alpha1.TaskStatus{{
			Name:   a.Pipeline.Spec.Tasks[l].Name,
			Status: &state,
		}}
	} else {
		l--
		taskRun := a.PipelineRun.Status.TaskRun[l]
		if *taskRun.Status == corev1.ConditionFalse ||
			*taskRun.Status == corev1.ConditionTrue {
			l += 1
			if l == len(a.Pipeline.Spec.Tasks) {
				// All task has finish.
				return nil
			}
			state := corev1.ConditionUnknown
			a.PipelineRun.Status.TaskRun = append(a.PipelineRun.Status.TaskRun, &v1alpha1.TaskStatus{
				Name:   a.Pipeline.Spec.Tasks[l].Name,
				Status: &state,
			})
		}

	}

	a.PipelineRun.Status.TaskRun[l].Name = a.Pipeline.Spec.Tasks[l].Name
	return &task{
		task:    a.Pipeline.Spec.Tasks[l],
		taskRun: a.PipelineRun.Status.TaskRun[l],

		pipelineRunStatus: &a.PipelineRun.Status,
	}
}

type task struct {
	task    v1alpha1.Task
	taskRun *v1alpha1.TaskStatus

	pipelineRunStatus *v1alpha1.PipeplineRunStatus
}

func (t *task) isStarted() bool {
	return t.taskRun.Status != nil && *t.taskRun.Status != ""
}

func (t *task) parseParams(params []v1alpha1.KeyAndValue) error {
	var getValue = func(params []v1alpha1.KeyAndValue, name string) *string {
		for _, param := range params {
			if param.Name == name {
				return param.Value
			}
		}
		return nil
	}

	var getOutputValue = func(taskStatus []*v1alpha1.TaskStatus, name string) *string {
		names := strings.Split(name, ".")
		if len(names) != 2 {
			return nil
		}
		for _, taskState := range taskStatus {
			if taskState.Name == names[0] {
				return getValue(taskState.Output, names[1])
			}
		}

		return nil
	}

	for i, param := range t.task.Params {
		switch {
		case strings.HasPrefix(*param.Value, "$(params."):
			name := (*param.Value)[9:]
			name = name[:len(name)-1]
			value := getValue(params, name)
			t.task.Params[i].Value = value
		case strings.HasPrefix(*param.Value, "$(task."):
			name := (*param.Value)[7:]
			name = name[:len(name)-1]
			value := getOutputValue(t.pipelineRunStatus.TaskRun, name)
			t.task.Params[i].Value = value
		}

		if t.task.Params[i].Value == nil {
			return fmt.Errorf("can not get the value of %s", *param.Value)
		}
	}
	return nil
}
