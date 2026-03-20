package notifier

import (
	"context"
	"errors"
	"time"
)

type Alert struct {
	Cluster      string
	Namespace    string
	ResourceKind string
	ResourceName string
	Reason       string
	Count        int
	Window       time.Duration
	Message      string
	Diagnosis    string
}

// Notifier sends alerts to an external system.
type Notifier interface {
	Send(ctx context.Context, alert Alert) error
}

// MultiNotifier fans out alerts to multiple notifiers.
type MultiNotifier struct {
	notifiers []Notifier
}

func NewMultiNotifier(notifiers ...Notifier) *MultiNotifier {
	return &MultiNotifier{notifiers: notifiers}
}

func (m *MultiNotifier) Send(ctx context.Context, alert Alert) error {
	var errs []error
	for _, n := range m.notifiers {
		if err := n.Send(ctx, alert); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
