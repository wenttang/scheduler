package scheduler

import (
	"sync"

	dapr "github.com/dapr/go-sdk/client"
	schedulerActor "github.com/wenttang/scheduler/pkg/actor"
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
