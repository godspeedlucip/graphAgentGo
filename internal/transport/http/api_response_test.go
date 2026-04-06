package http

import (
	"errors"
	"testing"

	appchat "go-sse-skeleton/internal/app/chat"
	domainchat "go-sse-skeleton/internal/domain/chat"
)

func TestMapHTTPError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "app invalid input",
			err:        &appchat.AppError{Code: appchat.ErrorCodeInvalidInput, Err: domainchat.ErrInvalidInput},
			wantStatus: 400,
			wantCode:   string(appchat.ErrorCodeInvalidInput),
		},
		{
			name:       "app not found",
			err:        &appchat.AppError{Code: appchat.ErrorCodeNotFound, Err: domainchat.ErrNotFound},
			wantStatus: 404,
			wantCode:   string(appchat.ErrorCodeNotFound),
		},
		{
			name:       "domain invalid input fallback",
			err:        domainchat.ErrInvalidInput,
			wantStatus: 400,
			wantCode:   string(domainchat.ErrorCodeInvalidInput),
		},
		{
			name:       "internal fallback",
			err:        errors.New("db down"),
			wantStatus: 500,
			wantCode:   string(domainchat.ErrorCodeInternal),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotStatus, gotCode, _ := mapHTTPError(tt.err)
			if gotStatus != tt.wantStatus || gotCode != tt.wantCode {
				t.Fatalf("unexpected mapping: got=(%d,%s), want=(%d,%s)", gotStatus, gotCode, tt.wantStatus, tt.wantCode)
			}
		})
	}
}
