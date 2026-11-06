//go:build live

package gpt

import (
	"testing"

	"neui.dev/internal/testkit/conform"
)

func TestConformLive(t *testing.T) {
	conform.RunLive(t, conformAdapter())
}
