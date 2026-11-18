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

package pending

import (
	"context"

	"github.com/doors-dev/doors"
	"neui.dev"
	"neui.dev/internal/common"
)

func Indicator(ctx context.Context) doors.Indicators {
	id := neui.IslandID(ctx)
	if id == "" {
		return nil
	}
	return doors.IndicateAttrQueryAll(submits(id), "aria-disabled", "true").
		And(doors.IndicateAttr("aria-busy", "true")).
		And(doors.IndicateAttr("aria-disabled", nil))
}

func FormIndicator(ctx context.Context) doors.Indicators {
	id := neui.IslandID(ctx)
	if id == "" {
		return nil
	}
	return doors.IndicateAttrQueryAll(submits(id), "aria-disabled", "true").
		And(doors.IndicateAttrSubmitter("aria-busy", "true")).
		And(doors.IndicateAttrSubmitter("aria-disabled", nil))
}

func submits(id string) string {
	return "[" + common.IslandIDAttr + `="` + id + `"] [data-neui="submit"]`
}
