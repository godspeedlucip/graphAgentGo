package orchestration

type StepStatus string

const (
	StepStatusPlanned  StepStatus = "planned"
	StepStatusExecuted StepStatus = "executed"
	StepStatusEvaluated StepStatus = "evaluated"
)

type FinalStatus string

const (
	FinalStatusDone     FinalStatus = "done"
	FinalStatusFailed   FinalStatus = "failed"
	FinalStatusCanceled FinalStatus = "canceled"
	FinalStatusTimeout  FinalStatus = "timeout"
)

type GraphInput struct {
	RunID        string
	AgentID      string
	SessionID    string
	UserInput    string
	SystemPrompt string
	MaxSteps     int
	Metadata     map[string]any
}

type GraphState struct {
	RunID      string
	AgentID    string
	SessionID  string
	StepIndex  int
	MaxSteps   int
	Stopped    bool
	Final      FinalStatus
	Reason     string
	FinalReply string
	Metadata   map[string]any
}

type SupervisorDecision struct {
	Action       string
	ToolName     string
	ToolInput    string
	DecisionType string
	Reason       string
	Confidence   float64
	Metadata     map[string]any
}

type WorkerOutput struct {
	Content  string
	Metadata map[string]any
}

type Evaluation struct {
	Pass            bool
	Score           float64
	Feedback        string
	Decision        string
	PolicyViolation bool
	Retryable       bool
	Metadata        map[string]any
}

type PolicyVerdict struct {
	Continue bool
	Reason   string
	Final    FinalStatus
}

type StepTrace struct {
	StepIndex   int
	Decision    SupervisorDecision
	Worker      WorkerOutput
	Evaluation  Evaluation
	Verdict     PolicyVerdict
}

type GraphResult struct {
	Status    FinalStatus
	Reason    string
	Reply     string
	StepsUsed int
	Trace     []StepTrace
}
