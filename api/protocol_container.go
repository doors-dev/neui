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

// Container is a tool output the agent gets as native content blocks on a
// direct call: the blocks Contents returns, in order. Everywhere else, in
// scripts and page tool results, the output is its JSON, which the output
// schema describes.
type Container interface {
	// Contents returns the blocks of the result, in order.
	Contents() []UserContent
}

// Document is a document a tool returns: raw bytes and their DocumentMime.
// Contents turns it into one ContentDocument with the mime as a plain string
// and no Name. Nothing checks Mime; check it with IsValid before returning.
type Document struct {
	// Data is the raw file bytes.
	Data []byte `json:"data"`
	// Mime is the document type; check it with IsValid before returning.
	Mime DocumentMime `json:"mime"`
}

// Contents returns d as one ContentDocument.
func (d Document) Contents() []UserContent {
	return []UserContent{ContentDocument{
		Data: d.Data,
		Mime: string(d.Mime),
	}}
}

var _ Container = Document{}

// Image is an image a tool returns: raw bytes and their ImageMime. Contents
// turns it into one ContentImage with the mime as a plain string. Nothing
// checks Mime; check it with IsValid before returning.
type Image struct {
	// Data is the raw image bytes.
	Data []byte `json:"data"`
	// Mime is the image type; check it with IsValid before returning.
	Mime ImageMime `json:"mime"`
}

// Contents returns i as one ContentImage.
func (i Image) Contents() []UserContent {
	return []UserContent{ContentImage{
		Data: i.Data,
		Mime: string(i.Mime),
	}}
}

var _ Container = Image{}

// ImageMime is an image type every built-in adapter accepts for a model that
// takes images.
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
// sends a document as a document where its API takes the type, as plain text
// for a text type (see IsText) where it does not, and as a note in its place
// when neither fits, such as for text whose bytes are not UTF-8.
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
