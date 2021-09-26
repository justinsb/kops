package resolver

import "context"

type Resolver interface {
	Resolve(ctx context.Context, host string) ([]string, error)
}
