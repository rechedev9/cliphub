package steamclient

import (
	"testing"

	"github.com/rechedev9/cliphub/internal/steamresolve"
)

func TestTransportSatisfiesInterface(t *testing.T) {
	var _ steamresolve.Transport = New(steamresolve.Session{})
}
