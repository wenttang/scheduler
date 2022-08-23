package scheduler

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-kit/log"
	"github.com/wenttang/scheduler/internal/scheduler/runtime"
	"github.com/wenttang/workflow/pkg/apis/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Actuator struct {
	Name        string
	Pipeline    *v1alpha1.Pipepline    `json:"pipeline,omitempty"`
	PipelineRun *v1alpha1.PipeplineRun `json:"pipeline_run,omitempty"`

	taskRunTime runtime.Runtime
	logger      log.Logger
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
		err := task.parseParams(task.task.Params, a.PipelineRun.Spec.Params)
		if err != nil {
			state := corev1.ConditionFalse
			task.taskRun.Status = &state
			return err
		}
	}

	return a.taskRunTime.Exec(ctx, task.task, task.taskRun)
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

func (a *Actuator) getTask() *taskSet {
	l := len(a.PipelineRun.Status.TaskRun)
	if l == 0 {
		a.PipelineRun.Status.TaskRun = []*v1alpha1.TaskStatus{{
			Name: a.Pipeline.Spec.Tasks[l].Name,
		}}
	} else {
		l--
		taskRun := a.PipelineRun.Status.TaskRun[l]
		if taskRun.Status == nil {
			return nil
		}
		if *taskRun.Status == corev1.ConditionFalse ||
			*taskRun.Status == corev1.ConditionTrue {
			l += 1
			if l == len(a.Pipeline.Spec.Tasks) {
				// All task has finish.
				return nil
			}
			a.PipelineRun.Status.TaskRun = append(a.PipelineRun.Status.TaskRun, &v1alpha1.TaskStatus{
				Name: a.Pipeline.Spec.Tasks[l].Name,
			})
		}
	}

	task := a.Pipeline.Spec.Tasks[l]
	taskRun := a.PipelineRun.Status.TaskRun[l]

	taskSet := &taskSet{
		task:              task,
		taskRun:           taskRun,
		pipelineRunStatus: &a.PipelineRun.Status,
	}

	if a.shouldSkip(taskSet) {
		state := corev1.ConditionTrue
		taskRun.Status = &state
		taskRun.Message = "Skip"
		return a.getTask()
	}

	return taskSet
}

func (a *Actuator) shouldSkip(task *taskSet) bool {
	var lookup = func(dependency string) bool {
		for _, elem := range a.PipelineRun.Status.TaskRun {
			if elem.Name == dependency {
				if elem.StartTime != nil {
					return true
				}
				break
			}
		}
		return false
	}

	for _, dependency := range task.task.Dependencies {
		if !lookup(dependency) {
			return true
		}
	}

	if len(task.task.When) != 0 {
		return a.shouldSkipByWhen(task)
	}

	return false
}

func (a *Actuator) shouldSkipByWhen(task *taskSet) bool {
	var parseValue = func(anyString *v1alpha1.AnyString) *v1alpha1.AnyString {
		key := anyString.String()
		if !strings.HasPrefix(key, "$(") {
			return anyString
		}

		tmp := []v1alpha1.KeyAndValue{{
			Value: anyString,
		}}
		task.parseParams(tmp, a.PipelineRun.Spec.Params)

		if tmp[0].Value != nil {
			return tmp[0].Value
		}
		return anyString
	}

	for i, when := range task.task.When {
		task.task.When[i].Input = parseValue(when.Input)
		for i, value := range when.Values {
			task.task.When[i].Values[i] = parseValue(value)
		}
	}

	var w when = task.task.When
	return w.sholdSkip()
}

type taskSet struct {
	task    v1alpha1.Task
	taskRun *v1alpha1.TaskStatus

	pipelineRunStatus *v1alpha1.PipeplineRunStatus
}

func (t *taskSet) isStarted() bool {
	return t.taskRun.Status != nil && *t.taskRun.Status != ""
}

func (t *taskSet) parseParams(dst, src []v1alpha1.KeyAndValue) error {
	var getValue = func(params []v1alpha1.KeyAndValue, name string) *v1alpha1.AnyString {
		for _, param := range params {
			if param.Name == name {
				return param.Value
			}
		}
		return nil
	}

	var getOutputValue = func(taskStatus []*v1alpha1.TaskStatus, name string) *v1alpha1.AnyString {
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

	for i, param := range dst {
		name := param.Value.String()
		switch {
		case strings.HasPrefix(name, "$(params."):
			name = name[9:]
			name = name[:len(name)-1]
			dst[i].Value = getValue(src, name)
		case strings.HasPrefix(name, "$(task."):
			name = name[7:]
			name = name[:len(name)-1]
			dst[i].Value = getOutputValue(t.pipelineRunStatus.TaskRun, name)
		}

		if dst[i].Value == nil {
			return fmt.Errorf("can not get the value of %s", *param.Value)
		}
	}
	return nil
}

type when []v1alpha1.When

// TODO:
func (w when) sholdSkip() bool {
	for _, elem := range w {
		switch elem.Operator {
		case "in":
			var in = func(elem *v1alpha1.AnyString, src []*v1alpha1.AnyString) bool {
				for _, value := range src {
					fmt.Println(value.String())
					if value.String() == elem.String() {
						return true
					}
				}
				return false
			}

			if !in(elem.Input, elem.Values) {
				return true
			}
		}
	}

	return false
}
