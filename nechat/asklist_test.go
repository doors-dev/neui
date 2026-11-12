package nechat

import (
	"context"
	"testing"
)

func TestAskModuleRegisters(t *testing.T) {
	m := ModuleAsk()(context.Background())
	t.Logf("module name=%q desc=%q", m.Name(), m.Desc())
	for _, tool := range m.List() {
		t.Logf("  tool name=%q", tool.Name())
	}
}
