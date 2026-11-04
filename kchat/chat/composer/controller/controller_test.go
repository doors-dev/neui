package controller

import (
	"context"
	"slices"
	"testing"

	"neui.dev/api"
)

type fakeThread struct {
	api.ChatThread
	sent    []api.Input
	rewound []api.Input
	inputs  []api.Input
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
	return NewComposerController(thread, api.ChatInput{}, false, api.EventUsage{}, nil, false, nil)
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
