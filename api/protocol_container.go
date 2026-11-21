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

// Container is a tool output that becomes one block of a result, the block
// Content returns; ContentType names it for the tool's description.
type Container interface {
	// Content returns the block the output becomes.
	Content() UserContent
	// ContentType names the kind of block, such as "image" or "document".
	ContentType() string
}

// MultiContainer is a tool output that becomes several blocks of one result,
// in the order Contents returns them.
type MultiContainer interface {
	// Contents returns the blocks of the result, in order.
	Contents() []UserContent
}

// Document is a document a tool returns: raw bytes and their DocumentMime.
// Content turns it into a ContentDocument with the mime as a plain string and
// no Name. Nothing checks Mime; check it with IsValid before returning.
type Document struct {
	// Data is the raw file bytes.
	Data []byte `json:"data"`
	// Mime is the document type; check it with IsValid before returning.
	Mime DocumentMime `json:"mime"`
}

// ContentType returns "document".
func (d Document) ContentType() string {
	return "document"
}

// Content returns d as a ContentDocument.
func (d Document) Content() UserContent {
	return ContentDocument{
		Data: d.Data,
		Mime: string(d.Mime),
	}
}

var _ Container = Document{}

// Image is an image a tool returns: raw bytes and their ImageMime. Content
// turns it into a ContentImage with the mime as a plain string. Nothing checks
// Mime; check it with IsValid before returning.
type Image struct {
	// Data is the raw image bytes.
	Data []byte `json:"data"`
	// Mime is the image type; check it with IsValid before returning.
	Mime ImageMime `json:"mime"`
}

// Content returns i as a ContentImage.
func (i Image) Content() UserContent {
	return ContentImage{
		Data: i.Data,
		Mime: string(i.Mime),
	}
}

// ContentType returns "image".
func (i Image) ContentType() string {
	return "image"
}

var _ Container = Image{}

// ImageMime is an image type the Claude and GPT adapters both accept. The
// Gemini adapter takes all four in messages and tool results.
type ImageMime string

// The accepted image types.
const (
	ImagePNG  ImageMime = "image/png"
	ImageJPEG ImageMime = "image/jpeg"
	ImageWEBP ImageMime = "image/webp"
	ImageGIF  ImageMime = "image/gif" // non-animated only: OpenAI rejects animation, Anthropic keeps frame 1
)

// IsValid reports whether m is one of the accepted image types.
func (m ImageMime) IsValid() bool {
	switch m {
	case ImagePNG, ImageJPEG, ImageWEBP, ImageGIF:
		return true
	default:
		return false
	}
}

// DocumentMime is a document type at least one provider accepts. An adapter
// sends a document in the best form its provider's API carries: a document
// where the API takes the type, plain text for a text type (DocumentText,
// DocumentMarkdown, DocumentCSV, DocumentHTML, DocumentXML, DocumentJSON,
// DocumentYAML, DocumentRTF) where it has no document form, and a note in its
// place when nothing fits, such as text whose bytes are not UTF-8.
type DocumentMime string

// The accepted document types. Only the text types with UTF-8 bytes reach the
// model on every provider, in a user message or a tool result.
const (
	DocumentPDF      DocumentMime = "application/pdf"
	DocumentText     DocumentMime = "text/plain"
	DocumentJSON     DocumentMime = "application/json"
	DocumentMarkdown DocumentMime = "text/markdown"
	DocumentCSV      DocumentMime = "text/csv"
	DocumentHTML     DocumentMime = "text/html"
	DocumentXML      DocumentMime = "text/xml"
	DocumentYAML     DocumentMime = "application/yaml"
	DocumentRTF      DocumentMime = "application/rtf"
	DocumentDOC      DocumentMime = "application/msword"
	DocumentXLS      DocumentMime = "application/vnd.ms-excel"
	DocumentDOCX     DocumentMime = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	DocumentXLSX     DocumentMime = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	DocumentPPTX     DocumentMime = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
)

// IsValid reports whether m is one of the accepted document types.
func (m DocumentMime) IsValid() bool {
	switch m {
	case DocumentPDF, DocumentText, DocumentJSON, DocumentMarkdown,
		DocumentCSV, DocumentHTML, DocumentXML, DocumentYAML,
		DocumentRTF, DocumentDOC, DocumentXLS, DocumentDOCX,
		DocumentXLSX, DocumentPPTX:
		return true
	default:
		return false
	}
}

// IsText reports whether m is a text type: DocumentText, DocumentMarkdown,
// DocumentCSV, DocumentHTML, DocumentXML, DocumentJSON, DocumentYAML or
// DocumentRTF.
func (m DocumentMime) IsText() bool {
	switch m {
	case DocumentText, DocumentMarkdown, DocumentCSV, DocumentHTML,
		DocumentXML, DocumentJSON, DocumentYAML, DocumentRTF:
		return true
	default:
		return false
	}
}
