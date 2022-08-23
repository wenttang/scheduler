package runtime

import (
	"context"
	"sync"

	dapr "github.com/dapr/go-sdk/client"
	"github.com/wenttang/scheduler/internal/scheduler/runtime"
	schedulerActor "github.com/wenttang/scheduler/pkg/actor"
	"github.com/wenttang/workflow/pkg/apis/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (t *Task) Exec(ctx context.Context, task v1alpha1.Task, status *v1alpha1.TaskStatus, opts ...runtime.Option) error {
	state := corev1.ConditionUnknown
	status.Status = &state

	actor := t.getActor(task.Actor.Type)
	params := make(map[string]interface{})
	for _, param := range task.Params {
		params[param.Name] = param.Value.GetValue()
	}
	resp, err := actor.ReconcileTask(ctx, params)

	if err != nil {
		return err
	}
	switch resp.Status {
	case schedulerActor.Running:
		state = corev1.ConditionUnknown
	case schedulerActor.True:
		now := metav1.Now()
		state = corev1.ConditionTrue
		status.CompletionTime = &now
	default:
		now := metav1.Now()
		state = corev1.ConditionFalse
		status.CompletionTime = &now
	}

	status.Status = &state
	status.Output = resp.OutPut
	return nil
}
