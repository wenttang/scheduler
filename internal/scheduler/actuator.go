package scheduler

import (
	"context"
	"fmt"
	"strings"

	"github.com/wenttang/workflow/pkg/apis/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

type Actuator struct {
	Pipeline    *v1alpha1.Pipepline
	PipelineRun *v1alpha1.PipeplineRun
}

func (a *Actuator) Reconcile(ctx context.Context) error {
	task := a.getTask()
	if task == nil {
		a.PipelineRun.Status.Status = corev1.ConditionTrue
		return nil
	}

	if !task.isStarted() {
		task.parseParams(a.PipelineRun.Spec.Params)
	}

	// TODO:

	return nil
}

func (a *Actuator) getTask() *task {
	l := len(a.PipelineRun.Status.TaskRun)
	if l == 0 {
		return &task{
			task: a.Pipeline.Spec.Tasks[0],
		}
	}

	taskRun := a.PipelineRun.Status.TaskRun[l-1]
	if *taskRun.Status == corev1.ConditionFalse ||
		*taskRun.Status == corev1.ConditionTrue {
		l += 1
	}

	l--
	if l == len(a.Pipeline.Spec.Tasks) {
		// All task has finish.
		return nil
	}

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
	return t.taskRun.Status != nil
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

	for _, param := range t.task.Params {
		if !strings.HasPrefix(*param.Value, "$(params.") {
			continue
		}

		name := (*param.Value)[9:]
		name = name[:len(name)-1]
		value := getValue(params, name)
		if value == nil {
			return fmt.Errorf("can not get the value of %s", *param.Value)
		}
		param.Value = value
	}
	return nil
}
