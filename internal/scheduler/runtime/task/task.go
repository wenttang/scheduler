package runtime

import (
	"context"
	"sync"
	"time"

	dapr "github.com/dapr/go-sdk/client"
	"github.com/wenttang/scheduler/internal/scheduler/runtime"
	schedulerActor "github.com/wenttang/scheduler/pkg/actor"
	"github.com/wenttang/scheduler/pkg/apis/v1alpha1"
)

type ActorSet struct {
	sync.RWMutex

	dapr   dapr.Client
	actors map[string]*schedulerActor.ClientStub
}

func (a *ActorSet) getActor(_t string) *schedulerActor.ClientStub {
	a.RLock()
	actor, ok := a.actors[_t]
	if ok {
		a.RUnlock()
		return actor
	}
	a.RUnlock()

	a.addActor(_t)
	return a.getActor(_t)
}

func (a *ActorSet) addActor(_t string) {
	actor := schedulerActor.New(a.dapr, "scheduler", _t)

	a.Lock()
	defer a.Unlock()
	a.actors[_t] = actor
}

type Task struct {
	ActorSet
}

func New(daprClient dapr.Client) runtime.Runtime {
	t := &Task{}

	t.actors = make(map[string]*schedulerActor.ClientStub)
	t.dapr = daprClient
	return t
}

func (t *Task) Exec(ctx context.Context, task v1alpha1.Task, taskStatus *v1alpha1.TaskStatus, opts ...runtime.Option) error {
	resp, err := t.exec(ctx, task, taskStatus)
	if err != nil {
		return err
	}
	taskStatus.Output = resp.OutPut

	state := v1alpha1.ConditionUnknown
	switch resp.Status {
	case schedulerActor.Running:
	case schedulerActor.True:
		state = v1alpha1.ConditionTrue
	default:
		state = v1alpha1.ConditionFalse
	}

	var subDone bool = true
	if task.WithItems != nil {
		subDone, err = t.gatherSubTask(taskStatus, &state)
	}

	if err != nil {
		return err
	}

	if subDone {
		taskStatus.Status = &state
		completionTime := time.Now()
		taskStatus.CompletionTime = &completionTime
	}

	return nil
}

func (t *Task) gatherSubTask(taskStatus *v1alpha1.TaskStatus, state *v1alpha1.ConditionStatus) (bool, error) {
	subTask := &taskStatus.SubTaskStatus[len(taskStatus.SubTaskStatus)-1]
	subTask.Status = state
	completionTime := time.Now()

	if *state == v1alpha1.ConditionFalse ||
		*state == v1alpha1.ConditionTrue {
		subTask.CompletionTime = &completionTime
	}

	slice, err := taskStatus.Items.GetSlice()
	if err != nil {
		return false, err
	}

	if len(slice) == len(taskStatus.SubTaskStatus) {
		return true, nil
	}

	return false, nil
}

func (t *Task) exec(ctx context.Context, task v1alpha1.Task, taskStatus *v1alpha1.TaskStatus) (*schedulerActor.ReconcileTaskResp, error) {
	actor := t.getActor(task.Actor.Type)
	params := make(map[string]interface{})
	for _, param := range task.Params {
		params[param.Name] = param.Value.GetValue()
	}
	return actor.ReconcileTask(ctx, params)
}
