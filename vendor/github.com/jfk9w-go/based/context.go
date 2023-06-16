package based

import "context"

type ContextFunc func(context.Context) (context.Context, context.CancelFunc)
