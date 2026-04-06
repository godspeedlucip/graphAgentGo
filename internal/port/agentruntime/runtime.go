package agentruntime

import "context"

type Runtime interface {
	Run(ctx context.Context) error
	AgentID() string
	SessionID() string
}