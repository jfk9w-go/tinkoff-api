package tinkoff

import "context"

type Authorizer interface {
	GetConfirmationCode(ctx context.Context, phone string) (string, error)
}

type authorizerKey struct{}

func WithAuthorizer(ctx context.Context, authorizer Authorizer) context.Context {
	return context.WithValue(ctx, authorizerKey{}, authorizer)
}

func getAuthorizer(ctx context.Context) Authorizer {
	authorizer, _ := ctx.Value(authorizerKey{}).(Authorizer)
	return authorizer
}
