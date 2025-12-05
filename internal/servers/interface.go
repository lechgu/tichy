package servers

import "context"

type WebServer interface {
	Run(ctx context.Context) error
}
