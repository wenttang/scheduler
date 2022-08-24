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
	taskSet := a.getTask()
	if taskSet == nil {
		a.PipelineRun.Status.Status = corev1.ConditionTrue
		return nil
	}

	if taskSet.canReentrancy() {
		err := a.parseTaskParams(taskSet)
		if err != nil {
			state := corev1.ConditionFalse
			taskSet.taskRun.Status = &state
			return err
		}

		if !taskSet.isStarted() {
			startTime := metav1.Now()
			state := corev1.ConditionUnknown
			taskSet.taskRun.StartTime = &startTime
			taskSet.taskRun.Status = &state
		}
	}

	return a.taskRunTime.Exec(ctx, taskSet.task, taskSet.taskRun)
}

func (a *Actuator) parseTaskParams(task *taskSet) error {
	if task.task.WithItems != nil {
		anyString := v1alpha1.AnyString(*task.task.WithItems)
		tmp := []v1alpha1.KeyAndValue{{
			Value: &anyString,
		}}
		err := task.parseParams(tmp, a.PipelineRun.Spec.Params)
		if err != nil {
			return err
		}

		task.taskRun.Items = tmp[0].Value

		// items must be slice
		_, err = task.taskRun.Items.GetSlice()
		if err != nil {
			return err
		}
	}

	err := task.parseParams(task.task.Params, a.PipelineRun.Spec.Params)
	if err != nil {
		return err
	}

	return nil
}

// ParseParams is parse pipeline params from pipelinerun.
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

// getTask returns the task being executed or the task that is about to be executed.
func (a *Actuator) getTask() *taskSet {
	var taskSet = &taskSet{}
	if len(a.PipelineRun.Status.TaskRun) == 0 ||
		*a.PipelineRun.Status.TaskRun[len(a.PipelineRun.Status.TaskRun)-1].Status == corev1.ConditionFalse ||
		*a.PipelineRun.Status.TaskRun[len(a.PipelineRun.Status.TaskRun)-1].Status == corev1.ConditionTrue {
		taskSet = a.getNextTask()
	} else {
		l := len(a.PipelineRun.Status.TaskRun) - 1
		taskSet.task = a.Pipeline.Spec.Tasks[l]
		taskSet.taskRun = a.PipelineRun.Status.TaskRun[l]
		taskSet.pipelineRunStatus = &a.PipelineRun.Status
	}

	if taskSet == nil {
		return nil
	}

	// If there are recurring subtasks,
	// overwrite the main task with the subtask
	if taskSet.task.WithItems != nil {
		a.mutateTask(taskSet)
	}

	return taskSet
}

// getNextTask return a new task. It will return nil, if all task has finsh.
// If task has sub-task, will return sub-task status.
func (a *Actuator) getNextTask() *taskSet {
	taskSet := &taskSet{}
	if len(a.PipelineRun.Status.TaskRun) == 0 {
		a.PipelineRun.Status.TaskRun = []*v1alpha1.TaskStatus{}
	}

	l := len(a.PipelineRun.Status.TaskRun)
	if l == len(a.Pipeline.Spec.Tasks) {
		// All task has finish.
		return nil
	}

	ts := &v1alpha1.TaskStatus{
		TaskStatusSpec: &v1alpha1.TaskStatusSpec{},
	}
	ts.Name = a.Pipeline.Spec.Tasks[l].Name
	a.PipelineRun.Status.TaskRun = append(a.PipelineRun.Status.TaskRun, ts)

	taskSet.task = a.Pipeline.Spec.Tasks[l]
	taskSet.taskRun = a.PipelineRun.Status.TaskRun[l]
	taskSet.pipelineRunStatus = &a.PipelineRun.Status

	if a.shouldSkip(taskSet) {
		state := corev1.ConditionTrue
		taskSet.taskRun.Status = &state
		taskSet.taskRun.Message = "Skip"
		return a.getNextTask()
	}

	return taskSet
}

func (a *Actuator) mutateTask(taskSet *taskSet) {
	if !taskSet.isStarted() {
		startTime := metav1.Now()
		taskSet.taskRun.SubTaskStatus = []v1alpha1.TaskStatusSpec{{
			StartTime: &startTime,
		}}
	}

	lastSubTaskStatus := taskSet.taskRun.SubTaskStatus[len(taskSet.taskRun.SubTaskStatus)-1]
	if lastSubTaskStatus.Status == nil ||
		*lastSubTaskStatus.Status == "" ||
		*lastSubTaskStatus.Status == corev1.ConditionUnknown {
		return
	}

	startTime := metav1.Now()
	taskSet.taskRun.SubTaskStatus = append(taskSet.taskRun.SubTaskStatus, v1alpha1.TaskStatusSpec{
		StartTime: &startTime,
	})
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

func (t *taskSet) canReentrancy() bool {
	if !t.isStarted() {
		return true
	}
	for _, sub := range t.taskRun.SubTaskStatus {
		if sub.Status == nil || *sub.Status == "" {
			return true
		}
	}

	return false
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
		case strings.HasPrefix(name, "$(item"):
			slice, _ := t.taskRun.Items.GetSlice()

			l := len(t.pipelineRunStatus.TaskRun[len(t.pipelineRunStatus.TaskRun)-1].SubTaskStatus) - 1
			dst[i].Value = v1alpha1.ParseAnyStringPtr(slice[l])
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
