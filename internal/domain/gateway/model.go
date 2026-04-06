package gateway

type Target string

const (
	TargetJava Target = "java"
	TargetGo   Target = "go"
)

type Request struct {
	Method string
	Path   string
	Header map[string]string
}

type Decision struct {
	Target Target
	Reason string
}

type Rule struct {
	Name         string
	PathPrefix   string
	Method       string
	Target       Target
	TrafficRatio int // 0-100, reserved for gradual rollout
	Enabled      bool
}

type Rules struct {
	DefaultTarget Target
	Items         []Rule
}