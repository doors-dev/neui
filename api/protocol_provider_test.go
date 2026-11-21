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

import "testing"

func TestModalityHas(t *testing.T) {
	all := ModalityText | ModalityImage | ModalityDocument
	for _, tc := range []struct {
		set, kind Modality
		want      bool
	}{
		{0, ModalityText, true},
		{0, ModalityImage, true},
		{0, ModalityImage | ModalityDocument, true},
		{ModalityText, ModalityText, true},
		{ModalityText, ModalityImage, false},
		{ModalityText, ModalityDocument, false},
		{ModalityText | ModalityImage, ModalityImage, true},
		{ModalityText | ModalityImage, ModalityDocument, false},
		{ModalityText | ModalityImage, ModalityImage | ModalityDocument, false},
		{all, ModalityImage | ModalityDocument, true},
		{ModalityImage, ModalityText, false},
	} {
		if got := tc.set.Has(tc.kind); got != tc.want {
			t.Errorf("Modality(%b).Has(%b) = %v, want %v", tc.set, tc.kind, got, tc.want)
		}
	}
}

func TestModelStaysComparable(t *testing.T) {
	a := Model{Name: "m", Inputs: ModalityText}
	b := a
	if a != b || !map[Model]bool{a: true}[b] {
		t.Fatal("two equal models compare unequal")
	}
	b.Inputs |= ModalityImage
	if a == b {
		t.Fatal("models that differ in Inputs compare equal")
	}
}
