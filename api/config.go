package api

import "time"

// ThreadConfig holds the tunables of an app that are not a provider, a storage
// or a module. A zero field takes its default; a caller sets only what it wants
// to change.
type ThreadConfig struct {
	// CompactionTrigger is the share of Model.Context that the thread's token
	// usage must reach, when a round ends, for the thread to start compacting.
	// It has no effect when Model.Context is zero, and above 1 it never fires;
	// the ThreadMemoryLimit byte budget and the API's over-length error still
	// start compaction either way. Default 0.8.
	CompactionTrigger float32
	// CompactionAllowed is the share of Model.Context the thread's token usage
	// must reach before a compaction the user requests is accepted; below it
	// the request fails with ErrWrongState. When Model.Context is zero the
	// share is not checked. Default 0.2.
	CompactionAllowed float32
	// CompactionPreserve is the share of the conversation kept verbatim after
	// the summary. Default 0.1.
	CompactionPreserve float32
	// ReplayResultsShare sets how much text each tool result keeps on replay:
	// its text blocks together are cut to
	// Model.Context * 4 * ReplayResultsShare / ToolCallLimit bytes (four bytes
	// per token; a non-positive ToolCallLimit divides by 1), or to 32 KiB when
	// Model.Context is zero. Media blocks are not cut. Negative replays every
	// result whole. The same cap bounds an island's ui.notify values between
	// draws, where a negative value counts as 1. Default 0.25.
	ReplayResultsShare float32
	// ThreadMemoryLimit is the byte budget over everything a thread holds for
	// replay: media counted raw, text counted as it stands. When
	// Model.RequestBytes is known the budget is at most
	// Model.RequestBytes*3/4 - ReplayHeadroom (the three quarters undo the
	// base64 inflation media takes on the wire). The first time the provider
	// refuses a request as too large without ErrMeasuredTooLarge, the budget
	// is cut to 80% and the round is retried. Going over the budget starts
	// compaction. Negative means no byte budget. Default 18 MiB.
	ThreadMemoryLimit int64
	// ToolCallLimit is how many tool calls of one round are executed; a call
	// past it is answered with an error result naming the limit, so every
	// tool_use still gets a matching tool_result. Negative runs every call a
	// round asks for. Default 32.
	ToolCallLimit int
	// InputBytes is the byte ceiling on one submitted Input: the length of the
	// text blocks Input.Content() renders, which are its Text plus the file
	// links its Attachments turn into. An input over it is refused with
	// ErrInputTooLarge where it is submitted, drafted or rewound to, so nothing
	// of it is drafted, stored or sent. It cannot be turned off: a
	// non-positive value takes the default, 256 KiB.
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
	// ReadLimit is the byte budget of one file read: lines are returned until
	// their lengths add up to it, the line that crosses it included, and the
	// read then reports where to continue from. Negative returns every
	// requested line. Default 50 KiB.
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
	// UnzipFiles is how many entries one extraction writes; at the next entry
	// it stops with an error, and the entries already written stay in the
	// workspace. Default 10000.
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
	// TurnTime is how long a script may run between ui.draw calls, fetch, app
	// function and tool calls included, before it is stopped. The app stays
	// up; the scripts still queued behind it are dropped. Default 30s.
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
	// FnMemory is the memory limit in bytes of the JS runtime an app run's
	// IslandCalls share, at least 4 MiB. Its call depth is MaxCallDepth.
	// Default 4 MiB.
	FnMemory int
}
