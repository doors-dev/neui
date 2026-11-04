package api

import "time"

// ThreadConfig holds the tunables of an app that are not a provider, a storage
// or a module. A zero field takes its default; a caller sets only what it wants
// to change.
type ThreadConfig struct {
	// CompactionTrigger is the share of the model's context window at which a
	// round is preceded by compaction. Above 1 it never fires, leaving the
	// API's over-length error as the only trigger. Default 0.8.
	CompactionTrigger float32
	// CompactionAllowed is the share of the model's context window the
	// conversation must fill before a compaction requested by the user is
	// accepted. Default 0.2.
	CompactionAllowed float32
	// CompactionPreserve is the share of the conversation kept verbatim after
	// the summary. Default 0.1.
	CompactionPreserve float32
	// ReplayResultsShare is the share of the model's context window, measured
	// in bytes, that all tool results of one round may occupy together. The
	// byte cap itself is derived where it is used, as
	// Model.Context * 4 * ReplayResultsShare / ToolCallLimit — four bytes per
	// token is this codebase's standing estimate — falling back to 32 KiB when
	// Model.Context is zero, so nothing caches it and a model with another
	// window changes the cap on the next round. Negative replays every result
	// whole. The same cap also sizes an island's ui.notify values between draws,
	// where a negative value counts as a full share. Default 0.25.
	ReplayResultsShare float32
	// ThreadMemoryLimit is the byte budget over everything a thread holds for
	// replay: media counted raw, text counted as it stands. The effective limit
	// is min(ThreadMemoryLimit, Model.RequestBytes*3/4 - ReplayHeadroom), where
	// the three quarters invert the base64 inflation media takes on the wire
	// and the headroom covers what the replay layer never sees. Negative means
	// no byte budget. Default 18 MiB.
	ThreadMemoryLimit int64
	// ToolCallLimit is how many tool calls of one round are executed; a call
	// past it is answered with an error result naming the limit, so every
	// tool_use still gets a matching tool_result. Negative runs every call a
	// round asks for. Default 32.
	ToolCallLimit int
	// InputBytes is the byte ceiling on one submitted Input: its Text plus the
	// file links its Attachments turn into, counted as Input.Content() renders
	// them. An input over it is refused with ErrInputTooLarge where it is
	// submitted, so nothing is drafted, stored or sent. The ceiling exists
	// because a single message too large for the model's context cannot be
	// recovered further down: compaction summarizes the history and keeps the
	// latest input whole, so it has nothing to shrink. Unlike the other byte
	// budgets here there is nothing downstream to fall back on and no way to
	// turn this one off, so a non-positive value takes the default, 256 KiB.
	InputBytes int
}

// ReplayHeadroom is what a thread's byte budget leaves free under the model's
// request limit for the parts of a request the replay layer cannot measure: the
// tool schemas and the JSON envelope are assembled past it, from the modules a
// thread runs and the provider adapter's own encoding, so replay subtracts a
// flat 1 MiB instead of counting them.
const ReplayHeadroom = 1 << 20

// FilesConfig bounds the workspace filesystem and the files module: what the
// workspace may hold on disk, how much of a file reaches the model, and how far
// an archive may unpack. A zero field takes its default and a negative field
// turns that limit off.
type FilesConfig struct {
	// MaxFiles is how many files the workspace may hold; the write that would
	// add one past it is refused with ErrQuota. Uploads live in file storage
	// and are not counted. Default 10000.
	MaxFiles int
	// MaxBytes is how many bytes the workspace may hold; a write crossing it is
	// refused with ErrQuota, at open or mid-write. Default 1 GiB.
	MaxBytes int64
	// ReadLimit is how many bytes one file read returns to the model; a longer
	// read stops there and reports where to continue from, leaving the file
	// untouched. Default 50 KiB.
	ReadLimit int
	// EditLimit is how many bytes of a file an edit may read into memory to
	// rewrite it; a bigger file is refused rather than loaded, since an edit
	// has to hold the whole file to replace text inside it. It bounds the
	// file, not the replacement, which the model's own output cap already
	// bounds. Default 10 MiB.
	EditLimit int
	// ImageFileLimit is how many bytes an image file may have to be handed to
	// the model; a bigger one is refused with ErrTooLarge rather than read as
	// an image. Default 20 MiB.
	ImageFileLimit int
	// DocumentLimit is how many bytes a document may have to be handed to the
	// model; a bigger one is refused with ErrTooLarge. Default 32 MiB.
	DocumentLimit int
	// UnzipLimit is the total unpacked size one archive may produce;
	// extraction stops and reports the limit once the entries written pass it,
	// so an archive cannot fill the workspace by what it decompresses to.
	// Default 512 MiB.
	UnzipLimit int64
	// UnzipFiles is how many entries one archive may hold; extraction is
	// refused past it. Default 10000.
	UnzipFiles int
}

// IslandConfig bounds the scripts that draw into islands: how much UI one
// app may hold, how much a script may queue between draws, and the JS
// runtime it runs on. A zero or negative field takes its default.
type IslandConfig struct {
	// MaxItems is how many slots, widgets and layouts one island may hold at
	// once; a create or set that would pass it is refused. Default 512.
	MaxItems int
	// MaxOps is how many sets, updates and notifies a script may queue before its
	// next ui.draw applies them; the call past it is refused. Default 512.
	MaxOps int
	// TurnTime is how long a script may run between ui.draw calls, fetch and
	// tool calls included, before it is stopped and its app ends. Default 30s.
	TurnTime time.Duration
	// Memory is the JS runtime's memory limit in bytes, at least 4 MiB.
	// Default 16 MiB.
	Memory int
	// MaxCallDepth is how many nested JS calls a script may make before a
	// stack overflow is thrown. Default 1000.
	MaxCallDepth int
	// MaxQueue is how many scripts sent with island_continue may wait to run;
	// the one past it is refused, unless it is sent with interrupt. Default 16.
	MaxQueue int
	// FnTime is how long one IslandCall may run, app function calls included,
	// before it is stopped. Default 5s.
	FnTime time.Duration
	// FnMemory is the memory limit in bytes of the fresh JS runtime each
	// IslandCall runs on, at least 4 MiB. Its call depth is MaxCallDepth.
	// Default 4 MiB.
	FnMemory int
}
