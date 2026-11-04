package files

import (
	"context"

	"neui.dev/internal/common"
)

const listCap = 1000

const listDesc = `List a directory of the thread filesystem: the workspace plus the virtual user_uploads/ and agent_uploads/ dirs at its root.`
const statDesc = `Read one entry's metadata.`
const copyDesc = `Copy a file. Copying to agent_uploads/<name> stores the file for the user; a taken name is an error, delete it first or pick another.`
const moveDesc = `Move or rename a file or directory. The uploads dirs are read-only.`
const deleteDesc = `Delete a file or directory recursively; missing is not an error. Deleting agent_uploads/<name> frees the name for a new copy; user_uploads/ is read-only.`

type listArg struct {
	Path string `json:"path" description:"directory to list, empty string is the workspace root"`
}

type listResult struct {
	Entries   []Entry `json:"entries"`
	Truncated bool    `json:"truncated,omitempty" description:"present when over 1000 entries exist, list a subdirectory instead"`
}

type copyArg struct {
	Src string `json:"src" required:"true"`
	Dst string `json:"dst" required:"true"`
}

type moveArg struct {
	From string `json:"from" required:"true"`
	To   string `json:"to" required:"true"`
}

type deleteResult struct {
	Deleted string `json:"deleted"`
}

func listTool(ctx context.Context, arg listArg) (listResult, error) {
	fs := common.GetController(ctx).FS()
	entries, err := fs.ReadDir(ctx, arg.Path)
	if err != nil {
		return listResult{}, err
	}
	res := listResult{Entries: make([]Entry, 0, min(len(entries), listCap))}
	for _, e := range entries {
		if len(res.Entries) == listCap {
			res.Truncated = true
			break
		}
		res.Entries = append(res.Entries, entry(e))
	}
	return res, nil
}

func statTool(ctx context.Context, arg PathArg) (Entry, error) {
	fs := common.GetController(ctx).FS()
	e, err := fs.Stat(ctx, arg.Path)
	if err != nil {
		return Entry{}, err
	}
	return entry(e), nil
}

func copyTool(ctx context.Context, arg copyArg) (Entry, error) {
	fs := common.GetController(ctx).FS()
	if err := fs.Copy(ctx, arg.Src, arg.Dst); err != nil {
		return Entry{}, err
	}
	e, err := fs.Stat(ctx, arg.Dst)
	if err != nil {
		return Entry{}, err
	}
	return entry(e), nil
}

func moveTool(ctx context.Context, arg moveArg) (Entry, error) {
	fs := common.GetController(ctx).FS()
	if err := fs.Rename(arg.From, arg.To); err != nil {
		return Entry{}, err
	}
	e, err := fs.Stat(ctx, arg.To)
	if err != nil {
		return Entry{}, err
	}
	return entry(e), nil
}

func deleteTool(ctx context.Context, arg PathArg) (deleteResult, error) {
	fs := common.GetController(ctx).FS()
	if err := fs.Remove(ctx, arg.Path); err != nil {
		return deleteResult{}, err
	}
	return deleteResult{Deleted: arg.Path}, nil
}
