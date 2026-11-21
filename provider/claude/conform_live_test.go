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

//go:build live

package claude

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/testkit/conform"
)

func TestConformLive(t *testing.T) {
	conform.RunLive(t, conformAdapter())
}

func TestLiveDeclaredInputs(t *testing.T) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		t.Skip("ANTHROPIC_API_KEY is not set")
	}
	for _, m := range conformAdapter().Models {
		t.Run(m.Name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, defaultBaseURL+"/v1/models/"+m.Name, nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("x-api-key", key)
			req.Header.Set("anthropic-version", defaultVersion)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Models API: %d %s", resp.StatusCode, body)
			}
			var info struct {
				Capabilities struct {
					ImageInput struct {
						Supported bool `json:"supported"`
					} `json:"image_input"`
					PDFInput struct {
						Supported bool `json:"supported"`
					} `json:"pdf_input"`
				} `json:"capabilities"`
			}
			if err := json.Unmarshal(body, &info); err != nil {
				t.Fatalf("%v: %s", err, body)
			}
			if got, want := m.Inputs.Has(api.ModalityImage), info.Capabilities.ImageInput.Supported; got != want {
				t.Errorf("Inputs %b reads images: %v, the Models API says image_input %v", m.Inputs, got, want)
			}
			if got, want := m.Inputs.Has(api.ModalityDocument), info.Capabilities.PDFInput.Supported; got != want {
				t.Errorf("Inputs %b reads documents: %v, the Models API says pdf_input %v", m.Inputs, got, want)
			}
		})
	}
}
