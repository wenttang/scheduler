package scheduler

import (
	"context"
	"errors"
	"fmt"

	"github.com/dapr/go-sdk/actor"
	dapr "github.com/dapr/go-sdk/client"
	workflowActor "github.com/wenttang/workflow/pkg/actor"
	corev1 "k8s.io/api/core/v1"
)

type Scheduler struct {
	actor.ServerImplBase
	daprClient dapr.Client
}

func New() actor.Server {
	client, err := dapr.NewClient()
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

	err := s.daprClient.RegisterActorTimer(ctx, &dapr.RegisterActorTimerRequest{
		ActorType: s.Type(),
		ActorID:   s.ID(),
		Name:      name,
		DueTime:   "5s",
		Period:    "5s",
		Data:      []byte(fmt.Sprintf(`"%s"`, name)),
		CallBack:  "Reconcile",
	})
	if err != nil {
		return err
	}

	req.PipelineRun.Status.Status = corev1.ConditionUnknown
	if err := s.GetStateManager().Set(name, &Actuator{
		Pipeline:    req.Pipeline,
		PipelineRun: req.PipelineRun,
	}); err != nil {
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

	return &workflowActor.Result{
		Status: actuator.PipelineRun.Status.Status,
	}, nil
}

func (s *Scheduler) Reconcile(ctx context.Context, name string) error {
	// TODO: pipeline params
	actuator := new(Actuator)
	err := s.GetStateManager().Get(name, actuator)
	if err != nil {
		return err
	}

	err = actuator.Reconcile(ctx)
	if err != nil {
		return err
	}

	// TODO:
	return nil
}
