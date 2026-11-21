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

package api

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestModalitiesHas(t *testing.T) {
	all := Modalities{ModalityText, ModalityImage, ModalityDocument}
	for _, tc := range []struct {
		set  Modalities
		kind Modality
		want bool
	}{
		{nil, ModalityText, true},
		{nil, ModalityImage, true},
		{nil, ModalityDocument, true},
		{Modalities{}, ModalityImage, true},
		{Modalities{}, ModalityDocument, true},
		{Modalities{ModalityText}, ModalityText, true},
		{Modalities{ModalityText}, ModalityImage, false},
		{Modalities{ModalityText}, ModalityDocument, false},
		{Modalities{ModalityText, ModalityImage}, ModalityImage, true},
		{Modalities{ModalityText, ModalityImage}, ModalityDocument, false},
		{all, ModalityImage, true},
		{all, ModalityDocument, true},
		{Modalities{ModalityImage}, ModalityText, true},
		{Modalities{ModalityImage}, ModalityDocument, false},
		{Modalities{ModalityDocument}, ModalityText, true},
		{Modalities{ModalityDocument}, ModalityImage, false},
	} {
		if got := tc.set.Has(tc.kind); got != tc.want {
			t.Errorf("%v.Has(%q) = %v, want %v", tc.set, tc.kind, got, tc.want)
		}
	}
}

func TestModalitiesJSON(t *testing.T) {
	data, err := json.Marshal(Model{Name: "m", Inputs: Modalities{ModalityText, ModalityImage}})
	if err != nil {
		t.Fatal(err)
	}
	var m Model
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(m.Inputs, Modalities{ModalityText, ModalityImage}) {
		t.Fatalf("%s reads back Inputs %v", data, m.Inputs)
	}
	data, err = json.Marshal(Modalities{ModalityText, ModalityImage, ModalityDocument})
	if err != nil || string(data) != `["text","image","document"]` {
		t.Fatalf("Modalities marshals as %s, %v", data, err)
	}
	for _, in := range []string{`{"inputs":null}`, `{"inputs":[]}`, `{}`} {
		var m Model
		if err := json.Unmarshal([]byte(in), &m); err != nil || len(m.Inputs) != 0 || !m.Inputs.Has(ModalityImage) {
			t.Errorf("%s reads back Inputs %v, %v, want empty, which holds every kind", in, m.Inputs, err)
		}
	}
	for _, in := range []string{`{"inputs":["text","audio"]}`, `{"inputs":["Image"]}`, `{"inputs":[""]}`, `{"inputs":[1]}`, `{"inputs":"text"}`} {
		var m Model
		if err := json.Unmarshal([]byte(in), &m); err == nil {
			t.Errorf("%s reads back Inputs %v, want an error", in, m.Inputs)
		}
	}
}
