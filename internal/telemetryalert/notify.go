package telemetryalert

import (
	"context"
	"fmt"
	"io"
)

// Notifier delivers one alert. It is the only seam between the rules and a
// channel, so a self-hosted ntfy can replace Telegram without rule changes.
type Notifier interface {
	Send(ctx context.Context, alert Alert) error
}

// Callback is an inline-button press (Ack / Resolver / Silenciar 24h).
type Callback struct {
	ID     string
	ChatID int64
	Data   string
}

// callbackSource is implemented by notifiers with interactive buttons.
type callbackSource interface {
	Callbacks(ctx context.Context, offset int64) ([]Callback, int64, error)
	Answer(ctx context.Context, callbackID, text string) error
}

// writerNotifier prints alerts; dry runs use it.
type writerNotifier struct{ w io.Writer }

func (n writerNotifier) Send(_ context.Context, alert Alert) error {
	silent := ""
	if alert.Priority == 2 || alert.Silent {
		silent = " (silenciosa)"
	}
	_, err := fmt.Fprintf(n.w, "--- %s%s\n%s\n", alert.Rule, silent, alert.Text())
	return err
}
