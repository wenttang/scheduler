package v1alpha1

// ActorRef dapr actor.
// // More info: https://docs.dapr.io/developing-applications/building-blocks/actors/
type ActorRef struct {
	Type string `json:"type,omitempty"`
}

type When struct {
	Input *AnyString `json:"input,omitempty"`

	Operator string `json:"operator,omitempty"`

	Values []*AnyString `json:"values,omitempty"`
}

// Task are run as part of a Pipeline using a set of inputs and producing a set of outputs.
type Task struct {
	Name string `json:"name,omitempty"`

	// Dependencies is a set of execution dependent tasks.
	// Only when all dependent tasks are completed can the current task be executed.
	// +optional
	Dependencies []string `json:"dependencies,omitempty"`

	// Params is a list of input parameters required to run the task.
	// +optional
	Params []KeyAndValue `json:"params,omitempty"`

	// +optional
	Output []*AnyString `json:"output,omitempty"`

	// +optional
	When []When `json:"when,omitempty"`

	// Actor is the executor of the task.
	Actor ActorRef `json:"actor,omitempty"`

	// WithItems is loop of the task.
	WithItems *string `json:"withItems,omitempty"`
}

// PipeplineSpec defines the desired state of Pipepline
type PipeplineSpec struct {
	// +optional
	Params []ParamSpec `json:"params,omitempty"`

	// Tasks is a collection of sequential task
	Tasks []Task `json:"tasks,omitempty"`
}

// PipeplineStatus defines the observed state of Pipepline
// TODO: Finalizer, modifying or deleting a pipeline should destroy
// all related pipelineRuns
type PipeplineStatus struct {
}

// Pipepline is the Schema for the pipeplines API
type Pipepline struct {
	ObjectMeta `json:"metadata,omitempty"`

	Spec   PipeplineSpec   `json:"spec,omitempty"`
	Status PipeplineStatus `json:"status,omitempty"`
}
