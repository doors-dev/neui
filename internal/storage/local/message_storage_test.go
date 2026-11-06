package local_test

import (
	"testing"

	"neui.dev/api"
	"neui.dev/internal/storage/local"
	"neui.dev/internal/testkit"
)

func TestLocalMessageStorage(t *testing.T) {
	testkit.MessageStorage(t, func(*testing.T) api.MessageStorage { return local.NewMessageStorage() })
}
