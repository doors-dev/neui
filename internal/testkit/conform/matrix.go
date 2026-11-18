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

package conform

import (
	"slices"
	"strings"

	"neui.dev/api"
)

func Matrix(ceiling int, withSampling bool) []Params {
	efforts := []string{"", EffortOff, EffortLow, EffortHigh, EffortMax}
	choices := []api.ToolChoice{"", api.ToolChoiceAuto, api.ToolChoiceNone, api.ToolChoiceAny, ChoiceNamed}
	sampling := [][2]float64{{0, 0}}
	if withSampling {
		sampling = append(sampling, [2]float64{0.5, 0}, [2]float64{0, 0.9}, [2]float64{0.5, 0.9})
	}
	tokens := []int{0, 1}
	if ceiling > 0 {
		tokens = append(tokens, ceiling, ceiling+1)
	}
	var out []Params
	for _, e := range efforts {
		for _, c := range choices {
			for _, s := range sampling {
				for _, n := range tokens {
					out = append(out, Params{Effort: e, Choice: c, Temperature: s[0], TopP: s[1], MaxTokens: n})
				}
			}
		}
	}
	return out
}

type expect struct {
	want   Outcome
	carry  map[string]any
	omit   []string
	bug    string
	source []string
}

func (a Adapter) expect(model, entry string, p Params) expect {
	e := expect{carry: map[string]any{}}
	for _, r := range a.Rules {
		if r.Model != "" && !strings.Contains(model, r.Model) {
			continue
		}
		if r.Entry != "" && r.Entry != entry {
			continue
		}
		if r.When != nil && !r.When(p) {
			continue
		}
		switch {
		case r.Want == RefuseNew, r.Want == RefuseLocal && e.want != RefuseNew, r.Want == Remote && e.want == Send:
			e.want = r.Want
		}
		for k, v := range r.Carry {
			e.carry[k] = v
		}
		e.omit = append(e.omit, r.Omit...)
		if r.Bug != "" {
			e.bug = r.Bug
		}
		e.source = append(e.source, r.Source)
	}
	return e
}

func (e expect) omits(ptr string) bool {
	return slices.Contains(e.omit, ptr)
}

func (p Params) wireValues() map[string]any {
	set := map[string]any{}
	switch p.Effort {
	case "":
	case EffortOff:
		set["off"] = nil
	default:
		set["effort"] = p.Effort
	}
	if p.Choice != "" {
		set["choice"] = nil
	}
	if p.Temperature != 0 {
		set["temperature"] = p.Temperature
	}
	if p.TopP != 0 {
		set["top_p"] = p.TopP
	}
	if p.MaxTokens != 0 {
		set["max_tokens"] = p.MaxTokens
	}
	return set
}

func (p Params) request(r api.Request) api.Request {
	r.ToolChoice = p.Choice
	return r
}
