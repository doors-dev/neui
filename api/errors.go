package api

import "errors"

// ErrDrift reports that a thread advanced between the read the caller worked
// from and the write it asked for, so nothing was written.
var ErrDrift = errors.New("thread advanced")

// ErrNotFound reports that no thread, message or record answers to the id the
// caller named.
var ErrNotFound = errors.New("not found")

// ErrExists reports a create on an id already taken, with nothing written; ids
// come from the caller, so a collision is a mistake upstream.
var ErrExists = errors.New("id already taken")

var ErrUnlinked = errors.New("instance was detached from the thread")
var ErrDisabled = errors.New("neui for this session is disabled")
var ErrWrongState = errors.New("thread is in the wrong state to perform this action")

// ErrNoProvider is what Enable returns when neither the app's WithProvider nor
// the session's OptionProvider sets a provider; the session stays as it was.
var ErrNoProvider = errors.New("no provider: set WithProvider on the app or pass OptionProvider to Enable")

// ErrCanceledByUser is the cause a tool or task context carries when the user
// stopped the round, cancelled the task from the UI, or rewound past it; read
// it with context.Cause. Any other cancellation is the runtime's own.
var ErrCanceledByUser = errors.New("canceled by user")

// ErrMeasuredTooLarge marks the CodeTooLarge an adapter reports when the server
// refused a request the adapter had already measured as fitting
// Model.RequestBytes: the declared limit over-promises for that endpoint, or the
// refusal is not about bytes at all. Neither is transient, so a caller reading
// it with errors.Is ends the turn instead of sending less and trying again, and
// the error names both the size measured and the limit it was measured against.
var ErrMeasuredTooLarge = errors.New("provider refused a request measured as fitting")

// ErrInputTooLarge answers an input over ThreadConfig.InputBytes, counting its text
// and the file links its attachments turn into. It is refused where it is
// submitted and leaves no trace: no draft is kept, no round is created, nothing
// is cut. Nothing downstream can rescue such a message — compaction summarizes
// the history and keeps the latest input whole — so the error names the two
// remedies a person has, and reaches them as written.
var ErrInputTooLarge = errors.New("message is too large to send, split it into smaller messages or attach the content as a file")

// ErrToolNotFound reports a tool call naming no registered tool.
var ErrToolNotFound = errors.New("tool not found")

// ErrReadOnly reports a write aimed at the uploads dirs.
var ErrReadOnly = errors.New("uploads dirs accept no direct writes; copy an upload into the workspace to edit it, copy to agent_uploads/<name> to store")

// ErrVirtual reports a local-path request for an entry that lives in file
// storage.
var ErrVirtual = errors.New("uploads live in file storage and have no local path; copy into the workspace first")

// ErrQuota reports a workspace write refused by the configured limits.
var ErrQuota = errors.New("workspace quota exceeded; delete files no longer needed to free space")

// ErrTooLarge reports a file refused for exceeding the configured size cap.
var ErrTooLarge = errors.New("file too large")

// ErrTooMany reports a file refused because the attachment count cap is
// reached.
var ErrTooMany = errors.New("too many files")

// ErrInvalidName reports a name the uploads dirs cannot hold: empty, dotted,
// nested, or aimed outside an uploads dir.
var ErrInvalidName = errors.New("invalid file name")

// ErrReserved reports a path aimed at the scratch directory TempDir hands to
// modules; it is not part of the workspace.
var ErrReserved = errors.New("that name is reserved for module scratch space and is not part of the workspace; use a different name")

// ErrOutOfRange reports a read offset at or past the end of the file.
var ErrOutOfRange = errors.New("read offset past end of file")

// ErrAskInIsland reports an Ask added to an island Library: an app function
// runs inside a script or a widget, where no one waits on a chat question.
var ErrAskInIsland = errors.New("an ask tool cannot be an island app function")
