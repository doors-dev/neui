package files

import (
	"context"

	"neui.dev/internal/common"
	"neui.dev/internal/modules/files/text"
	"neui.dev/internal/tools"
)

const moduleDesc = `The workspace is your scratch filesystem: plain relative paths, wiped when the thread closes. Two flat virtual dirs persist in file storage: user_uploads/ holds the user's attached files, read-only; agent_uploads/ is your persistent store — copy to agent_uploads/<name> to keep a file across sessions, delete to free the name. Copy a stored file into the workspace to edit it or when a tool needs a local path. Only stored files can be shared into the chat: a markdown link like [report.pdf](file:///agent_uploads/report.pdf) or [notes.txt](file:///user_uploads/notes.txt) renders as a download, ![chart](file:///agent_uploads/chart.png) renders an image inline. Workspace paths never render — copy a file to agent_uploads/<name> first to share it. Nothing outside the uploads dirs is guaranteed to survive: the workspace is cleared when the thread exits, so store any artifact you may need later — MEMORY.md, notes, generated results — in agent_uploads/.`

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
