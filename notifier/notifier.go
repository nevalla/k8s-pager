package notifier

import "context"

// Notifier sends alerts to an external system.
type Notifier interface {
	Send(ctx context.Context, alert Alert) error
}
