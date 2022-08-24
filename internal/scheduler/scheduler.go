package scheduler

import (
	"context"
	"fmt"

	"github.com/dapr/go-sdk/actor"
	dapr "github.com/dapr/go-sdk/client"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/wenttang/scheduler/internal/config"
	"github.com/wenttang/scheduler/internal/scheduler/runtime"
	taskRuntime "github.com/wenttang/scheduler/internal/scheduler/runtime/task"
	"github.com/wenttang/scheduler/pkg/middleware"
	workflowActor "github.com/wenttang/workflow/pkg/actor"
	"github.com/wenttang/workflow/pkg/apis/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

type Scheduler struct {
	actor.ServerImplBase
	conf       *config.Config
	middleware *middleware.Chain

	taskRunTime runtime.Runtime
	daprClient  dapr.Client
	logger      log.Logger
}

func New(conf config.Config, logger log.Logger, daprClient dapr.Client) func() actor.Server {
	logger = log.With(logger, "Module", "Scheduler")

	return func() actor.Server {
		return &Scheduler{
			conf: &conf,

			daprClient: daprClient,
			logger:     logger,
			middleware: middleware.New(daprClient, logger, conf.Middleware.Pre, conf.Middleware.Post),

			taskRunTime: taskRuntime.New(daprClient),
		}
	}
}

func (s *Scheduler) Type() string {
	return "scheduler"
}

func (s *Scheduler) Register(ctx context.Context, req *workflowActor.RegisterReq) (*workflowActor.Result, error) {
	if req.Pipeline == nil || req.PipelineRun == nil {
		return s.returnWithFailedMessage("Failed get pipeline or pipelineRun")
	}

	name := fmt.Sprintf("%s:%s", req.PipelineRun.Namespace, req.PipelineRun.Name)
	if exist, err := s.GetStateManager().Contains(s.getStateName(name)); err != nil {
		level.Info(s.logger).Log("message", err.Error())
		return s.returnWithFailedMessage(err.Error())
	} else if exist {
		return s.returnWithFailedMessage("Expecting to register, actually already exists")
	}

	actuator := &Actuator{
		Name:        name,
		Pipeline:    req.Pipeline,
		PipelineRun: req.PipelineRun,
	}

	err := actuator.ParseParams()
	if err != nil {
		return s.returnWithFailedMessage(err.Error())
	}

	req.PipelineRun.Status.Status = corev1.ConditionUnknown
	err = s.GetStateManager().Set(s.getStateName(name), actuator)
	if err != nil {
		return s.returnWithFailedMessage(err.Error())
	}

	if err := s.saveAndFlush(); err != nil {
		return s.returnWithFailedMessage(err.Error())
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
		return s.returnWithFailedMessage(err.Error())
	}

	level.Info(s.logger).Log("message", "Success", "name", name)
	return &workflowActor.Result{
		Status: corev1.ConditionUnknown,
	}, nil
}

func (s *Scheduler) GetStatus(ctx context.Context, req *workflowActor.NamespacedName) (*workflowActor.Result, error) {
	actuator := new(Actuator)
	name := fmt.Sprintf("%s:%s", req.Namespace, req.Name)

	level.Info(s.logger).Log("message", "Get status", "name", name)
	err := s.GetStateManager().Get(s.getStateName(name), actuator)
	if err != nil {
		return s.returnWithFailedMessage(err.Error())
	}

	if actuator.PipelineRun == nil || actuator.Pipeline == nil {
		return s.returnWithFailedMessage(err.Error())
	}

	return &workflowActor.Result{
		Status:  actuator.PipelineRun.Status.Status,
		Reason:  actuator.PipelineRun.Status.Reason,
		Message: actuator.PipelineRun.Status.Message,
		TaskRun: actuator.PipelineRun.Status.TaskRun,
	}, nil
}

func (s *Scheduler) Clear(ctx context.Context, req *workflowActor.NamespacedName) error {
	name := fmt.Sprintf("%s:%s", req.Namespace, req.Name)

	level.Info(s.logger).Log("message", "Try to clear", "name", name)
	if exist, err := s.GetStateManager().Contains(s.getStateName(name)); err != nil {
		level.Info(s.logger).Log("message", err.Error())
		return nil
	} else if exist {
		return nil
	}

	err := s.GetStateManager().Remove(s.getStateName(name))
	if err != nil {
		level.Info(s.logger).Log("message", err.Error())
		return nil
	}

	err = s.saveAndFlush()
	if err != nil {
		level.Info(s.logger).Log("message", err.Error())
		return nil
	}

	level.Info(s.logger).Log("message", "Clear success", "name", name)
	return nil
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
	actuator.taskRunTime = s.taskRunTime

	mwReq := &middleware.DoReq{
		Pipeline:    actuator.Pipeline,
		PipelineRun: actuator.PipelineRun,
	}
	s.middleware.Pre(ctx, mwReq)
	defer s.middleware.Post(ctx, mwReq)
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
	if actuator.Pipeline == nil || actuator.PipelineRun == nil {
		level.Info(s.logger).Log("message", "can not get pipeline or pipelineRun")
		return true
	}

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
	defer func(actuator *Actuator, pipipeline *v1alpha1.Pipepline) {
		actuator.Pipeline = pipipeline
	}(actuator, actuator.Pipeline.DeepCopy())

	err := actuator.Reconcile(ctx)
	if err != nil {
		level.Error(s.logger).Log("message", err.Error())
		reason := v1alpha1.SchedulerFail
		actuator.PipelineRun.Status.Reason = &reason
		actuator.PipelineRun.Status.Message = err.Error()
		actuator.PipelineRun.Status.Status = corev1.ConditionFalse

		return true
	}

	if actuator.PipelineRun.Status.IsFinish() {
		level.Info(s.logger).Log("message", "All task finish or some task failed")
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

func (s *Scheduler) returnWithFailedMessage(message string) (*workflowActor.Result, error) {
	level.Info(s.logger).Log("message", message)
	reason := v1alpha1.SchedulerFail
	return &workflowActor.Result{
		Status:  corev1.ConditionFalse,
		Reason:  &reason,
		Message: message,
	}, nil
}
