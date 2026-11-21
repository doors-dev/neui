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

const moduleDesc = `The workspace is temporary and takes plain relative paths, like report.md. user_uploads/ holds the files the user attached, read-only: an attachment arrives as a link like [notes.txt](file:///user_uploads/notes.txt), and its path is user_uploads/notes.txt. agent_uploads/ holds the files you give the user, and they stay with this conversation. Neither takes direct writes. To change an upload, copy it into the workspace and edit the copy. To give the user a file, write it in the workspace, copy it to agent_uploads/<name> and link it in your reply: [report.md](file:///agent_uploads/report.md) shows a download, ![chart](file:///agent_uploads/chart.png) shows the image. Copy anything you may need later, such as notes or results, to agent_uploads/<name> too. read shows line breaks as \n, the tools that change a file take them as \n, and a changed file keeps its own CRLF or LF endings and BOM.`

func Module(ctx context.Context) common.Module {
	tier := common.Tier(ctx)
	m := tools.NewModule("files", moduleDesc)
	add(m, "list", listDesc, listTool)
	add(m, "read", readDesc(tier), readTool)
	add(m, "readImage", readImageDesc, readImageTool)
	add(m, "readDocument", readDocumentDesc, readDocumentTool)
	add(m, "copy", copyDesc, copyTool)
	add(m, "delete", deleteDesc, deleteTool)
	add(m, "grep", grepDesc, grepTool)
	if tier != common.TierAssistant {
		add(m, "stat", statDesc, statTool)
		add(m, "readBytes", readBytesDesc, readBytesTool)
		add(m, "writeBytes", writeBytesDesc, writeBytesTool)
		add(m, "move", moveDesc, moveTool)
		add(m, "zip", zipDesc, zipTool)
		add(m, "unzip", unzipDesc, unzipTool)
		add(m, "glob", globDesc, globTool)
	}
	text.Register(m, tier)
	return m
}

func add[I any, O any](m tools.Module, name string, desc string, f func(ctx context.Context, input I) (O, error)) {
	if _, err := m.Add(name, desc, f); err != nil {
		panic(err)
	}
}
