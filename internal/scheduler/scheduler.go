package scheduler

import (
	"context"
	"errors"
	"fmt"

	"github.com/dapr/go-sdk/actor"
	dapr "github.com/dapr/go-sdk/client"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	schedulerActor "github.com/wenttang/scheduler/pkg/actor"
	workflowActor "github.com/wenttang/workflow/pkg/actor"
	corev1 "k8s.io/api/core/v1"
)

type Scheduler struct {
	actor.ServerImplBase
	daprClient dapr.Client

	actorSet *ActorSet
	logger   log.Logger
}

func New(logger log.Logger, darpClient dapr.Client) func() actor.Server {
	logger = log.With(logger, "Module", "Scheduler")
	return func() actor.Server {
		return &Scheduler{
			actorSet: &ActorSet{
				actors: make(map[string]*schedulerActor.ClientStub),
				dapr:   darpClient,
			},
			daprClient: darpClient,
			logger:     logger,
		}
	}
}

func (s *Scheduler) Type() string {
	return "scheduler"
}

func (s *Scheduler) Register(ctx context.Context, req *workflowActor.RegisterReq) error {
	if req.Pipeline == nil || req.PipelineRun == nil {
		level.Info(s.logger).Log("message", "Failed get pipeline or pipelineRun")
		return errors.New("invalid register")
	}

	name := fmt.Sprintf("%s:%s", req.PipelineRun.Namespace, req.PipelineRun.Name)
	if exist, err := s.GetStateManager().Contains(s.getStateName(name)); err != nil {
		level.Info(s.logger).Log("message", err.Error())
		return err
	} else if exist {
		level.Info(s.logger).Log("message", "Expecting to register, actually already exists", "name", name)
		return errors.New("already exists")
	}

	actuator := &Actuator{
		Name:        name,
		Pipeline:    req.Pipeline,
		PipelineRun: req.PipelineRun,
	}

	err := actuator.ParseParams()
	if err != nil {
		level.Info(s.logger).Log("message", err.Error(), "name", name)
		return err
	}

	req.PipelineRun.Status.Status = corev1.ConditionUnknown
	err = s.GetStateManager().Set(s.getStateName(name), actuator)
	if err != nil {
		level.Error(s.logger).Log("message", err.Error())
		return err
	}

	if err := s.saveAndFlush(); err != nil {
		level.Error(s.logger).Log("message", "Failed save state")
		return err
	}

	err = s.daprClient.RegisterActorReminder(ctx, &dapr.RegisterActorReminderRequest{
		ActorType: s.Type(),
		ActorID:   s.ID(),
		Name:      name,
		DueTime:   "5s",
		Period:    "30s",
		Data:      []byte(name),
	})
	if err != nil {
		level.Error(s.logger).Log("message", err.Error())
		return err
	}

	level.Info(s.logger).Log("message", "Success", "name", name)
	return nil
}

func (s *Scheduler) Get(ctx context.Context, req *workflowActor.NamespacedName) (*workflowActor.Result, error) {
	actuator := new(Actuator)
	name := fmt.Sprintf("%s:%s", req.Namespace, req.Name)
	err := s.GetStateManager().Get(name, actuator)
	if err != nil {
		return &workflowActor.Result{}, err
	}

	if actuator.PipelineRun == nil {
		return &workflowActor.Result{}, errors.New("not exists")
	}

	return &workflowActor.Result{
		Status: actuator.PipelineRun.Status.Status,
	}, nil
}

func (s *Scheduler) ReminderCall(reminderName string, state []byte, dueTime string, period string) {
	name := string(state)
	level.Info(s.logger).Log("message", "ReminderCall", "name", name)

	ctx := context.Background()
	s.reminderCall(ctx, name)
}

func (s *Scheduler) reminderCall(ctx context.Context, name string) error {
	var stopReminder = func(s *Scheduler, actuator *Actuator) error {
		err := s.daprClient.UnregisterActorReminder(ctx, &dapr.UnregisterActorReminderRequest{
			ActorType: s.Type(),
			ActorID:   s.ID(),
			Name:      name,
		})
		if err != nil {
			level.Error(s.logger).Log("message", err.Error(), "name", name)
			return err
		}
		level.Info(s.logger).Log("message", "Successed unregister actor timer", "name", name)
		return err
	}

	actuator := &Actuator{}
	err := s.GetStateManager().Get(s.getStateName(name), actuator)
	if err != nil {
		level.Error(s.logger).Log("message", err.Error())
		actuator.Name = name
		err := stopReminder(s, actuator)
		if err != nil {
			level.Error(s.logger).Log("message", "Failed stop reminder")
			return err
		}
		return nil
	}

	actuator.logger = log.With(s.logger, "name", name)
	actuator.ActorSet = s.actorSet
	isDone := s.Reconcile(ctx, actuator)
	if isDone {
		err := stopReminder(s, actuator)
		if err != nil {
			level.Error(s.logger).Log("message", "Failed stop reminder")
			return err
		}
	}

	level.Info(s.logger).Log("message", "Successed timer call", "name", name)
	return nil
}

func (s *Scheduler) Reconcile(ctx context.Context, actuator *Actuator) bool {
	defer func(s *Scheduler, actuator *Actuator) {
		err := s.GetStateManager().Set(s.getStateName(actuator.Name), actuator)
		if err != nil {
			level.Error(s.logger).Log("message", err.Error())
			return
		}
		err = s.saveAndFlush()
		if err != nil {
			level.Error(s.logger).Log("message", err.Error())
			return
		}
	}(s, actuator)

	if actuator.Pipeline == nil || actuator.PipelineRun == nil {
		level.Info(s.logger).Log("message", "can not get pipeline or pipelineRun")
		return true
	}

	err := actuator.Reconcile(ctx)
	if err != nil {
		level.Error(s.logger).Log("message", err.Error())
		return true
	}

	if actuator.PipelineRun.Status.IsFinish() {
		level.Info(s.logger).Log("message", "All task finish or some task fialed")
		return true
	}

	return false
}

func (s *Scheduler) getStateName(name string) string {
	return fmt.Sprintf("%s||save", name)
}

func (s *Scheduler) saveAndFlush() error {
	err := s.GetStateManager().Save()
	if err != nil {
		level.Error(s.logger).Log("message", err.Error())
		return err
	}
	s.GetStateManager().Flush()
	return nil
}
