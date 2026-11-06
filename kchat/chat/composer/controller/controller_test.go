package controller

import (
	"context"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"neui.dev/api"
)

type fakeThread struct {
	api.ChatThread
	inputBytes int
	sent       []api.Input
	rewound    []api.Input
	inputs     []api.Input
}

func (f *fakeThread) Config() api.ThreadConfig {
	if f.inputBytes == 0 {
		return api.ThreadConfig{InputBytes: inputLimit}
	}
	return api.ThreadConfig{InputBytes: f.inputBytes}
}

func (f *fakeThread) Send(ctx context.Context, input api.Input) error {
	f.sent = append(f.sent, input)
	return nil
}

func (f *fakeThread) Rewind(ctx context.Context, round api.RoundID, input api.Input) error {
	f.rewound = append(f.rewound, input)
	return nil
}

func (f *fakeThread) Input(ctx context.Context, input api.Input) error {
	f.inputs = append(f.inputs, input)
	return nil
}

func newTestController(thread *fakeThread) ComposerController {
	return NewComposerController(thread, api.ChatInput{}, false, api.EventUsage{}, nil, false, false, nil)
}

func TestSendRefusedWhileUploading(t *testing.T) {
	ctx := context.Background()
	thread := &fakeThread{}
	c := newTestController(thread)
	c.UpdateInputText(ctx, "hello")
	seq := c.origin.Get().inputSeq
	c.UploadStarted(ctx, seq)
	c.Send(ctx)
	if len(thread.sent) != 0 {
		t.Fatalf("Send during an upload: want nothing sent, got %v", thread.sent)
	}
	c.UploadFinished(ctx, seq)
	c.Send(ctx)
	if len(thread.sent) != 1 {
		t.Fatalf("Send after the upload: want one send, got %v", thread.sent)
	}

	c.InitRewind(ctx, 1, api.Input{Text: "edit"})
	seq = c.origin.Get().inputSeq
	c.UploadStarted(ctx, seq)
	c.Send(ctx)
	if len(thread.rewound) != 0 {
		t.Fatalf("rewind Send during an upload: want nothing sent, got %v", thread.rewound)
	}
	c.UploadFinished(ctx, seq)
	c.Send(ctx)
	if len(thread.rewound) != 1 {
		t.Fatalf("rewind Send after the upload: want one rewind, got %v", thread.rewound)
	}
}

func TestSeqBumpResetsUploading(t *testing.T) {
	ctx := context.Background()
	for name, bump := range map[string]func(c ComposerController){
		"InitRewind": func(c ComposerController) { c.InitRewind(ctx, 1, api.Input{Text: "edit"}) },
		"CancelRewind": func(c ComposerController) {
			c.InitRewind(ctx, 1, api.Input{Text: "edit"})
			c.UploadStarted(ctx, c.origin.Get().inputSeq)
			c.CancelRewind(ctx)
		},
		"Ask": func(c ComposerController) { c.Ask(ctx, api.ChatAsk{}) },
	} {
		t.Run(name, func(t *testing.T) {
			c := newTestController(&fakeThread{})
			c.UploadStarted(ctx, c.origin.Get().inputSeq)
			old := c.origin.Get().inputSeq
			bump(c)
			c.UploadFinished(ctx, old)
			if got := c.origin.Get().uploading; got != 0 {
				t.Fatalf("uploading after %s: want 0, got %d", name, got)
			}
		})
	}
}

func TestStaleUploaderAttachmentsIgnored(t *testing.T) {
	ctx := context.Background()
	thread := &fakeThread{}
	c := newTestController(thread)
	old := c.origin.Get().inputSeq
	c.InitRewind(ctx, 1, api.Input{Text: "edit", Attachments: []string{"a.txt"}})
	c.UpdateAttachments(ctx, old, []string{"stale.txt"})
	if got := c.origin.Get().rewind.input.Attachments; !slices.Equal(got, []string{"a.txt"}) {
		t.Fatalf("rewind attachments after a stale update: want [a.txt], got %v", got)
	}
	c.UpdateAttachments(ctx, c.origin.Get().inputSeq, []string{"a.txt", "b.txt"})
	if got := c.origin.Get().rewind.input.Attachments; !slices.Equal(got, []string{"a.txt", "b.txt"}) {
		t.Fatalf("rewind attachments after a current update: want [a.txt b.txt], got %v", got)
	}
}

func TestOverLimitTextSurvivesAttachmentRemoval(t *testing.T) {
	ctx := context.Background()
	thread := &fakeThread{}
	c := newTestController(thread)
	seq := c.origin.Get().inputSeq
	c.UpdateInputText(ctx, "short")
	c.UpdateAttachments(ctx, seq, []string{"user_uploads/a.txt"})
	over := strings.Repeat("x", inputLimit+1)
	c.UpdateInputText(ctx, over)
	c.UpdateAttachments(ctx, seq, nil)
	if c.enabled.Get() {
		t.Fatalf("Send enabled with %d bytes of text over a %d limit after removing the attachment", len(over), inputLimit)
	}
	c.Send(ctx)
	if len(thread.sent) != 0 {
		t.Fatalf("Send with over-limit text: want nothing sent, got %v", thread.sent)
	}
	for _, in := range thread.inputs {
		if len(in.Text) > inputLimit-len("short") {
			t.Fatalf("over-limit text reached thread.Input: %d bytes", len(in.Text))
		}
	}
}

func TestFittingTextSentAfterAttachmentRemoval(t *testing.T) {
	ctx := context.Background()
	thread := &fakeThread{}
	c := newTestController(thread)
	seq := c.origin.Get().inputSeq
	c.UpdateAttachments(ctx, seq, []string{"user_uploads/a.txt"})
	text := strings.Repeat("y", inputLimit-5)
	c.UpdateInputText(ctx, text)
	if c.enabled.Get() {
		t.Fatalf("Send enabled with text and attachment over the limit")
	}
	c.UpdateAttachments(ctx, seq, nil)
	if !c.enabled.Get() {
		t.Fatalf("Send disabled after removing the attachment")
	}
	c.Send(ctx)
	if len(thread.sent) != 1 || thread.sent[0].Text != text || len(thread.sent[0].Attachments) != 0 {
		t.Fatalf("Send after removing the attachment: want the latest text, got %d sends", len(thread.sent))
	}
	if n := len(thread.inputs); n == 0 || thread.inputs[n-1].Text != text {
		t.Fatalf("thread.Input after removing the attachment: want the latest text")
	}
}

func TestTrimmedTextSentExactly(t *testing.T) {
	ctx := context.Background()
	thread := &fakeThread{}
	c := newTestController(thread)
	c.UpdateInputText(ctx, strings.Repeat("x", inputLimit+10))
	c.UpdateInputText(ctx, "trimmed")
	if !c.enabled.Get() {
		t.Fatalf("Send disabled after trimming the text below the limit")
	}
	c.Send(ctx)
	if len(thread.sent) != 1 || thread.sent[0].Text != "trimmed" {
		t.Fatalf("Send after trimming: want the visible text, got %v", thread.sent)
	}
}

func TestCutKeepsRunes(t *testing.T) {
	ctx := context.Background()
	c := newTestController(&fakeThread{})
	text := strings.Repeat("x", inputLimit-1) + "\u00e9"
	c.UpdateInputText(ctx, text)
	got := c.origin.Get().input.Text
	if !utf8.ValidString(got) || got != text[:inputLimit-1] {
		t.Fatalf("cut text: want %d valid bytes, got %d (valid %v)", inputLimit-1, len(got), utf8.ValidString(got))
	}
	if c.origin.Get().textBytes != len(text) {
		t.Fatalf("textBytes: want %d, got %d", len(text), c.origin.Get().textBytes)
	}
}

func TestVisibleInputResetsSizes(t *testing.T) {
	ctx := context.Background()
	thread := &fakeThread{}
	c := newTestController(thread)
	c.UpdateInputText(ctx, strings.Repeat("a", inputLimit+1))
	c.InitRewind(ctx, 1, api.Input{Text: "short"})
	c.Send(ctx)
	if len(thread.rewound) != 1 {
		t.Fatalf("rewind of a short message after an over-limit draft: want one rewind, got %d", len(thread.rewound))
	}

	c.CancelRewind(ctx)
	c.UpdateInputText(ctx, "draft")
	c.InitRewind(ctx, 2, api.Input{Text: "edit"})
	c.UpdateInputText(ctx, strings.Repeat("b", inputLimit+1))
	c.CancelRewind(ctx)
	c.Send(ctx)
	if len(thread.sent) != 1 {
		t.Fatalf("main draft after an over-limit rewind: want one send, got %d", len(thread.sent))
	}

	c.UpdateInputText(ctx, strings.Repeat("c", inputLimit-10))
	c.FlushInput(ctx)
	before := len(thread.inputs)
	c.UpdateAttachments(ctx, c.origin.Get().inputSeq, []string{"user_uploads/report.pdf"})
	if len(thread.inputs) != before+1 {
		t.Fatalf("attachment after a flushed near-limit draft: want the draft saved")
	}

	restored := NewComposerController(thread, api.ChatInput{Input: api.Input{Text: strings.Repeat("d", inputLimit+1)}}, false, api.EventUsage{}, nil, false, false, nil)
	if restored.ExcessBytes().Get() == 0 {
		t.Fatalf("restored over-limit draft: want excess bytes")
	}
}

func TestLimitFollowsThreadInputBytes(t *testing.T) {
	ctx := context.Background()
	thread := &fakeThread{inputBytes: 1000}
	c := newTestController(thread)
	c.UpdateInputText(ctx, strings.Repeat("a", 1001))
	if got := c.ExcessBytes().Get(); got != 1 {
		t.Fatalf("1001 bytes over a 1000-byte thread limit: want 1 excess byte, got %d", got)
	}
	c.Send(ctx)
	if len(thread.sent) != 0 {
		t.Fatalf("over the thread limit: want no send, got %d", len(thread.sent))
	}

	wide := &fakeThread{inputBytes: 4 * inputLimit}
	c = newTestController(wide)
	c.UpdateInputText(ctx, strings.Repeat("a", inputLimit+1))
	if got := c.ExcessBytes().Get(); got != 1 {
		t.Fatalf("thread limit above the UI ceiling: want the ceiling to apply, got %d excess bytes", got)
	}
}

func TestCanCompactFollowsEvents(t *testing.T) {
	ctx := context.Background()
	thread := &fakeThread{}
	c := NewComposerController(thread, api.ChatInput{}, false, api.EventUsage{}, nil, false, true, nil)
	if !c.Context().Get().CanCompact {
		t.Fatalf("constructor: want CanCompact")
	}
	c.UpdateCompaction(ctx, true, api.EventUsage{}, false)
	if c.Context().Get().CanCompact {
		t.Fatalf("compaction started: want no CanCompact")
	}
	c.UpdateCompaction(ctx, false, api.EventUsage{Input: 10}, true)
	if !c.Context().Get().CanCompact {
		t.Fatalf("compaction failed: want CanCompact")
	}
	c.RoundDone(ctx, api.RoundError, api.EventUsage{}, false)
	if c.Context().Get().CanCompact {
		t.Fatalf("round end: want CanCompact from the event")
	}
	usage := api.EventUsage{Input: 7}
	c.RoundStart(ctx, usage, true)
	got := c.Context().Get()
	if !got.CanCompact || got.Usage != usage || !c.Active().Get() {
		t.Fatalf("round start: want active, CanCompact and the event usage, got %+v active=%v", got, c.Active().Get())
	}
}
