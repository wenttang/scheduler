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
		task.parseParams(a.PipelineRun.Spec.Params)
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
	return nil
}

func (a *Actuator) ParseParams() error {
	var getParams = func(params []v1alpha1.Param, name string) *v1alpha1.Param {
		for _, param := range params {
			if param.Name == name {
				return &param
			}
		}
		return nil
	}

	result := make([]v1alpha1.Param, 0, len(a.Pipeline.Spec.Params))
	for _, paramSpec := range a.Pipeline.Spec.Params {
		param := getParams(a.PipelineRun.Spec.Params, paramSpec.Name)
		if param == nil {
			if paramSpec.Default == nil {
				return fmt.Errorf("%s is required", param.Name)
			}
			param = &v1alpha1.Param{
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
		a.PipelineRun.Status.TaskRun = []*v1alpha1.TaskStatus{{}}
	} else {
		l--
		taskRun := a.PipelineRun.Status.TaskRun[l]
		if *taskRun.Status == corev1.ConditionFalse ||
			*taskRun.Status == corev1.ConditionTrue {
			l += 1
			a.PipelineRun.Status.TaskRun = append(a.PipelineRun.Status.TaskRun, &v1alpha1.TaskStatus{})
		}

		if l == len(a.Pipeline.Spec.Tasks) {
			// All task has finish.
			return nil
		}
	}

	a.PipelineRun.Status.TaskRun[l].Name = a.Pipeline.Spec.Tasks[l].Name
	return &task{
		task:    a.Pipeline.Spec.Tasks[l],
		taskRun: a.PipelineRun.Status.TaskRun[l],
	}
}

type task struct {
	task    v1alpha1.Task
	taskRun *v1alpha1.TaskStatus
}

func (t *task) isStarted() bool {
	return t.taskRun.Status != nil && *t.taskRun.Status != ""
}

func (t *task) parseParams(params []v1alpha1.Param) error {
	var getValue = func(params []v1alpha1.Param, name string) *string {
		for _, param := range params {
			if param.Name == name {
				return param.Value
			}
		}
		return nil
	}

	for i, param := range t.task.Params {
		if !strings.HasPrefix(*param.Value, "$(params.") {
			continue
		}

		name := (*param.Value)[9:]
		name = name[:len(name)-1]
		value := getValue(params, name)
		if value == nil {
			return fmt.Errorf("can not get the value of %s", *param.Value)
		}
		t.task.Params[i].Value = value
	}
	return nil
}
