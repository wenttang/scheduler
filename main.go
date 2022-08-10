package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/dapr/go-sdk/actor"
	dapr "github.com/dapr/go-sdk/client"
	daprd "github.com/dapr/go-sdk/service/http"
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
		CallBack:  "Next",
	})
	if err != nil {
		return err
	}

	req.PipelineRun.Status.Status = corev1.ConditionUnknown
	if err := s.GetStateManager().Set(name, req); err != nil {
		return err
	}
	fmt.Println("Regsiter actor timer")
	return nil
}

func (s *Scheduler) Get(ctx context.Context, req *workflowActor.NamespacedName) (*workflowActor.Result, error) {
	state := new(workflowActor.RegisterReq)
	name := fmt.Sprintf("%s:%s", req.Namespace, req.Name)
	err := s.GetStateManager().Get(name, state)
	if err != nil {
		return &workflowActor.Result{}, err
	}

	return &workflowActor.Result{
		Status: state.PipelineRun.Status.Status,
	}, nil
}

func (s *Scheduler) Next(ctx context.Context, name string) error {

	fmt.Println("----------", name)
	return nil
}

func main() {
	s := daprd.NewService(":8090")
	s.RegisterActorImplFactory(New)
	if err := s.Start(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("error listenning: %v", err)
	}
}
