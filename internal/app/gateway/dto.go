package gateway

type RouteRequest struct {
	Method string
	Path   string
	Header map[string]string
}

type RouteResult struct {
	Target string
	Reason string
}