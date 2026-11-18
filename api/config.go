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
	// ReplayItemShare sets the round item budget, Model.Context * 4 *
	// ReplayItemShare bytes (four bytes per token), assuming a 200,000-token
	// context when Model.Context is zero. Input: an Input whose text blocks,
	// its Text plus the file links its Attachments turn into, add up to more
	// than the budget is refused with ErrInputTooLarge where it is submitted,
	// drafted or rewound to. Tool results: a round's calls run in order while
	// their results' text is under the budget; the result that crosses it is
	// kept whole and the calls after it are answered without running; a result
	// whose text alone is over it is replaced by a note giving its size.
	// Records: a round takes the waiting records in arrival order while their
	// bodies add up to at most the budget, the rest wait for the next round,
	// and a first record over it alone is cut to it and taken by itself. On
	// replay each result's text, each record's body and the input's text are
	// cut to the budget, keeping head and tail; an input is over it only when
	// the budget shrank after it was sent, as after a switch to a smaller
	// model. Media blocks are not counted. Half of it bounds the values an
	// island script queues with ui.notify between draws. A non-positive value
	// takes the default, 0.1; a value above 0.25 counts as 0.25.
	ReplayItemShare float32
	// ThreadMemoryLimit is the byte budget over everything a thread holds for
	// replay: media counted raw, text counted as it stands. When
	// Model.RequestBytes is known the budget is at most
	// Model.RequestBytes*3/4 - ReplayHeadroom (the three quarters undo the
	// base64 inflation media takes on the wire). The first request the
	// provider refuses as too large lowers the budget and retries the round.
	// Going over the budget starts compaction. Negative means no byte budget.
	// Default 18 MiB.
	ThreadMemoryLimit int64
	// ToolCallLimit is how many tool calls of one round are executed; a call
	// past it is not run and is answered with an error naming the limit.
	// Negative runs every call a round asks for. Default 32.
	ToolCallLimit int
}

// ReplayHeadroom is the part of the model's request limit, 1 MiB, that a
// thread's byte budget leaves free for the tool schemas and the request
// envelope, which the budget does not count.
const ReplayHeadroom = 1 << 20

// FilesConfig bounds the workspace filesystem and the files module: what the
// workspace may hold on disk, how much of a file reaches the model, and how far
// an archive may unpack. A zero field takes its default and a negative field
// turns that limit off.
type FilesConfig struct {
	// TempDir is where each session creates its own temporary directory,
	// which holds the thread workspaces and, unless the session has its own
	// file storage, the uploads. It is read when Enable first runs for the
	// session. Default os.TempDir().
	TempDir string
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
	// MaxQueue is how many of the agent's scripts may wait to run on one
	// island; the one past it is refused, unless the agent sends it to replace
	// the running and queued ones. Default 16.
	MaxQueue int
	// FnTime is how long one IslandCall may run, app function calls included,
	// before it is stopped. Default 5s.
	FnTime time.Duration
	// FnMemory is the memory limit in bytes of the JS runtime an app run's
	// IslandCalls share, at least 4 MiB. Its call depth is MaxCallDepth.
	// Default 4 MiB.
	FnMemory int
}
