// Copyright 2026 doors dev LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package neui_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"neui.dev"
	"neui.dev/internal/common"
	chatmod "neui.dev/internal/modules/chat"
	"neui.dev/internal/modules/files"
	timemod "neui.dev/internal/modules/time"
	"neui.dev/internal/page"
	"neui.dev/nekit"
)

type noSearch struct{}

func (noSearch) Search(context.Context, neui.SearchQuery) ([]neui.SearchResult, error) {
	return nil, nil
}

func TestBuiltInToolsHaveSimpleSchemas(t *testing.T) {
	factories := []neui.ModuleFactory{
		chatmod.Module,
		timemod.Module,
		files.Module,
		neui.ModuleJS,
		neui.ModuleXLS(neui.XLSConfig{}),
		neui.ModuleWeb(neui.WebConfig{Search: noSearch{}}),
		nekit.ModuleAsk(),
		func(ctx context.Context) common.Module { return page.NewPage(ctx).Module(ctx) },
	}
	for _, tier := range []neui.ModelTier{neui.TierDeveloper, neui.TierAgent, neui.TierAssistant} {
		ctx := context.WithValue(context.Background(), common.TierKey{}, tier)
		for _, factory := range factories {
			m := factory(ctx)
			if m == nil {
				continue
			}
			for _, tool := range m.List() {
				simple := tool.Input().Simple
				if simple == nil {
					t.Errorf("tier %d: %s__%s has no simple input schema: %s", tier, m.Name(), tool.Name(), tool.Input().Raw)
					continue
				}
				if !json.Valid(simple) || bytes.Contains(simple, []byte(`"$ref"`)) || bytes.Contains(simple, []byte(`"definitions"`)) || bytes.Contains(simple, []byte(`"$defs"`)) {
					t.Errorf("tier %d: %s__%s has the simple input schema %s", tier, m.Name(), tool.Name(), simple)
				}
			}
		}
	}
}
