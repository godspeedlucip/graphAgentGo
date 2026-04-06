package gateway

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	domain "go-sse-skeleton/internal/domain/gateway"
)

type ReverseProxy struct {
	javaBaseURL string
	goBaseURL   string
	client      *http.Client
}

func NewReverseProxy(javaBaseURL string, goBaseURL string, client *http.Client) (*ReverseProxy, error) {
	if javaBaseURL == "" || goBaseURL == "" || client == nil {
		return nil, errors.New("invalid reverse proxy config")
	}
	return &ReverseProxy{javaBaseURL: javaBaseURL, goBaseURL: goBaseURL, client: client}, nil
}

func (p *ReverseProxy) ForwardHTTP(ctx context.Context, target domain.Target, req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, domain.ErrInvalidInput
	}
	base, err := p.pickBaseURL(target)
	if err != nil {
		return nil, err
	}
	urlStr, err := joinURL(base, req.URL.Path, req.URL.RawQuery)
	if err != nil {
		return nil, err
	}

	var body io.Reader
	if req.Body != nil {
		body = req.Body
	}
	newReq, err := http.NewRequestWithContext(ctx, req.Method, urlStr, body)
	if err != nil {
		return nil, err
	}
	newReq.Header = req.Header.Clone()
	return p.client.Do(newReq)
}

func (p *ReverseProxy) ForwardSSE(ctx context.Context, target domain.Target, w http.ResponseWriter, req *http.Request) error {
	if req == nil || w == nil {
		return domain.ErrInvalidInput
	}
	resp, err := p.ForwardHTTP(ctx, target, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	flusher, ok := w.(http.Flusher)
	if !ok {
		return errors.New("response writer does not support flush")
	}

	buf := make([]byte, 4096)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			flusher.Flush()
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return readErr
		}
	}
}

func (p *ReverseProxy) pickBaseURL(target domain.Target) (string, error) {
	switch target {
	case domain.TargetJava:
		return p.javaBaseURL, nil
	case domain.TargetGo:
		return p.goBaseURL, nil
	default:
		return "", domain.ErrNoRouteMatched
	}
}

func joinURL(base string, path string, rawQuery string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(path, "/")
	u.RawQuery = rawQuery
	return u.String(), nil
}