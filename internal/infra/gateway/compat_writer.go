package gateway

import (
	"io"
	"net/http"
)

type CompatWriter struct{}

func NewCompatWriter() *CompatWriter {
	return &CompatWriter{}
}

func (w *CompatWriter) WriteError(rw http.ResponseWriter, err error) {
	http.Error(rw, err.Error(), http.StatusBadGateway)
}

func (w *CompatWriter) CopyUpstreamResponse(rw http.ResponseWriter, resp *http.Response) error {
	for k, vv := range resp.Header {
		for _, v := range vv {
			rw.Header().Add(k, v)
		}
	}
	rw.WriteHeader(resp.StatusCode)
	_, err := io.Copy(rw, resp.Body)
	return err
}
