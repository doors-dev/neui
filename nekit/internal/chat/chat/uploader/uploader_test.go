package uploader

import (
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/nekit/internal/chat/params"
)

func TestCountingReaderRefusesPastLimit(t *testing.T) {
	exact := &countingReader{r: strings.NewReader(strings.Repeat("x", 10)), limit: 10}
	if n, err := io.Copy(io.Discard, exact); err != nil || n != 10 {
		t.Fatalf("file at the limit: want 10 bytes and no error, got %d, %v", n, err)
	}
	over := &countingReader{r: strings.NewReader(strings.Repeat("x", 11)), limit: 10}
	if _, err := io.Copy(io.Discard, over); !errors.Is(err, api.ErrTooLarge) {
		t.Fatalf("file over the limit: want api.ErrTooLarge, got %v", err)
	}
}

type seqController struct {
	seqs []int
	last []string
}

func (c *seqController) UpdateAttachments(ctx context.Context, seq int, files []string) {
	c.seqs = append(c.seqs, seq)
	c.last = files
}

func (c *seqController) UploadStarted(ctx context.Context, seq int) {}

func (c *seqController) UploadFinished(ctx context.Context, seq int) {}

func TestUploaderReportsItsSeq(t *testing.T) {
	c := &seqController{}
	u := New([]string{"a.txt"}, nil, c, 7, params.Params{MaxAttachments: 5})
	u.inflight = 1
	u.add(context.Background(), "b.txt")
	u.remove(context.Background(), "a.txt")
	if !slices.Equal(c.seqs, []int{7, 7}) {
		t.Fatalf("seqs reported: want [7 7], got %v", c.seqs)
	}
	if !slices.Equal(c.last, []string{"b.txt"}) {
		t.Fatalf("attachments: want [b.txt], got %v", c.last)
	}
}
