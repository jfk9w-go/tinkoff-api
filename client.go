package tinkoff

import (
	"context"
	"net/http"

	"github.com/jfk9w-go/based"
)

type SessionStorage interface {
	LoadSession(ctx context.Context, phone string) (string, error)
	UpdateSession(ctx context.Context, phone, session string) error
}

type ConfirmationProvider interface {
	GetConfirmationCode(ctx context.Context, phone string) (string, error)
}

type Client struct {
	httpClient           *http.Client
	sessionCache         *based.WriteThroughCache[string, string]
	confirmationProvider ConfirmationProvider
}
