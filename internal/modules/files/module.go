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

package files

import (
	"context"

	"neui.dev/internal/common"
	"neui.dev/internal/modules/files/text"
	"neui.dev/internal/tools"
)

const moduleDesc = `The workspace is your scratch filesystem: plain relative paths, wiped when the thread closes. Two flat virtual dirs at its root persist in file storage: user_uploads/ holds the user's attached files, read-only; agent_uploads/ is your persistent store — copy anything you may need later, such as MEMORY.md, notes or generated results, to agent_uploads/<name> to keep it across sessions. Copy a stored file into the workspace to edit it or when a tool needs a local path. Only stored files can be shared into the chat, so copy a workspace file to agent_uploads/<name> first. A markdown link like [report.pdf](file:///agent_uploads/report.pdf) or [notes.txt](file:///user_uploads/notes.txt) renders as a download, and ![chart](file:///agent_uploads/chart.png) renders an image inline. The read tool shows line breaks as \n; write, append, edit, replace and insert take them as \n, and a file they change keeps its own CRLF or LF endings and BOM.`

func Module(context.Context) common.Module {
	m := tools.NewModule("files", moduleDesc)
	add(m, "list", listDesc, listTool)
	add(m, "stat", statDesc, statTool)
	add(m, "read", readDesc, readTool)
	add(m, "readBytes", readBytesDesc, readBytesTool)
	add(m, "writeBytes", writeBytesDesc, writeBytesTool)
	add(m, "readImage", readImageDesc, readImageTool)
	add(m, "readDocument", readDocumentDesc, readDocumentTool)
	add(m, "copy", copyDesc, copyTool)
	add(m, "move", moveDesc, moveTool)
	add(m, "delete", deleteDesc, deleteTool)
	add(m, "zip", zipDesc, zipTool)
	add(m, "unzip", unzipDesc, unzipTool)
	add(m, "grep", grepDesc, grepTool)
	add(m, "glob", globDesc, globTool)
	text.Register(m)
	return m
}

func add[I any, O any](m tools.Module, name string, desc string, f func(ctx context.Context, input I) (O, error)) {
	if _, err := m.Add(name, desc, f); err != nil {
		panic(err)
	}
}
