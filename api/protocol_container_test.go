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

func TestDocumentMimeIsText(t *testing.T) {
	for mime, want := range map[DocumentMime]bool{
		DocumentText:      true,
		DocumentMarkdown:  true,
		DocumentCSV:       true,
		DocumentHTML:      true,
		DocumentXML:       true,
		DocumentJSON:      true,
		DocumentYAML:      true,
		DocumentRTF:       true,
		DocumentPDF:       false,
		DocumentDOC:       false,
		DocumentXLS:       false,
		DocumentDOCX:      false,
		DocumentXLSX:      false,
		DocumentPPTX:      false,
		"":                false,
		"application/xml": false,
		"text/x-go":       false,
		" text/plain":     false,
		"Text/Plain":      false,
	} {
		if got := mime.IsText(); got != want {
			t.Errorf("DocumentMime(%q).IsText() = %v, want %v", mime, got, want)
		}
	}
}
