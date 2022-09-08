package runtime

import (
	"context"

	"github.com/wenttang/scheduler/pkg/apis/v1alpha1"
)

type Option func(Runtime)

type Runtime interface {
	Exec(context.Context, v1alpha1.Task, *v1alpha1.TaskStatus, ...Option) error
}
