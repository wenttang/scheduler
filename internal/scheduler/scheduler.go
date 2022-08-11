package scheduler

import (
	"context"
	"errors"
	"fmt"

	"github.com/dapr/go-sdk/actor"
	dapr "github.com/dapr/go-sdk/client"
	schedulerActor "github.com/wenttang/scheduler/pkg/actor"
	workflowActor "github.com/wenttang/workflow/pkg/actor"
	corev1 "k8s.io/api/core/v1"
)

type Scheduler struct {
	actor.ServerImplBase
	daprClient dapr.Client
}

func New() actor.Server {
	client, err := schedulerActor.NewDapr()
	if err != nil {
		panic(err)
	}

	return &Scheduler{
		daprClient: client,
	}
}

func (s *Scheduler) Type() string {
	return "scheduler"
}

func (s *Scheduler) Register(ctx context.Context, req *workflowActor.RegisterReq) error {
	if req.Pipeline == nil || req.PipelineRun == nil {
		return errors.New("invalid register")
	}

	name := fmt.Sprintf("%s:%s", req.PipelineRun.Namespace, req.PipelineRun.Name)
	if exist, err := s.GetStateManager().Contains(name); err != nil {
		return err
	} else if exist {
		return errors.New("already exists")
	}

	actuator := &Actuator{
		dapr:        s.daprClient,
		Pipeline:    req.Pipeline,
		PipelineRun: req.PipelineRun,
	}

	err := actuator.ParseParams()
	if err != nil {
		return err
	}

	err = s.daprClient.RegisterActorReminder(ctx, &dapr.RegisterActorReminderRequest{
		ActorType: s.Type(),
		ActorID:   s.ID(),
		Name:      name,
		DueTime:   "30s",
		Period:    "30s",
		Data:      []byte(fmt.Sprintf(`"%s"`, name)),
		// CallBack:  "Reconcile",
	})
	if err != nil {
		return err
	}

	req.PipelineRun.Status.Status = corev1.ConditionUnknown
	if err := s.GetStateManager().Set(name, actuator); err != nil {
		return err
	}
	fmt.Println("Regsiter actor timer")
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
	ctx := context.Background()
	s.Reconcile(ctx, name)

	s.GetStateManager().Save()
}

func (s *Scheduler) Reconcile(ctx context.Context, name string) error {
	actuator := new(Actuator)
	err := s.GetStateManager().Get(name, actuator)
	if err != nil {
		return err
	}

	if actuator.Pipeline == nil || actuator.PipelineRun == nil {
		return nil
	}

	err = actuator.Reconcile(ctx)
	if err != nil {
		return err
	}

	return nil
}
