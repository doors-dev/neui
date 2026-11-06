package lib

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"unsafe"

	"neui.dev/api"
)

type bodyTransport string

func (t bodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(t))), Request: req}, nil
}

// Each frame's data buffer goes back to the reader once Convert returns and
// carries a later frame, which a converter that copies what it keeps never
// sees: every frame still arrives whole, and an empty data: line still
// dispatches nothing even when it lands in a reused buffer.
func TestFrameDataIsReusedAfterConvert(t *testing.T) {
	var body strings.Builder
	var want []string
	for i := range 200 {
		switch i % 4 {
		case 0:
			fmt.Fprintf(&body, "event: e%d\ndata: %s\n\n", i, strings.Repeat("x", i*37+1))
			want = append(want, fmt.Sprintf("e%d|%s", i, strings.Repeat("x", i*37+1)))
		case 1:
			fmt.Fprintf(&body, "data: a%d\ndata: b%d\n\n", i, i)
			want = append(want, fmt.Sprintf("|a%d\nb%d", i, i))
		case 2:
			body.WriteString("data:\n\n: comment\n\n")
		case 3:
			fmt.Fprintf(&body, "data: %d\n\n", i)
			want = append(want, fmt.Sprintf("|%d", i))
		}
	}
	body.WriteString("data: stop\n\n")
	var got []string
	buffers := map[*byte]bool{}
	c := conv{convert: func(f Frame) ([]api.Event, bool) {
		if string(f.Data) == "stop" {
			return nil, false
		}
		buffers[unsafe.SliceData(f.Data)] = true
		got = append(got, f.Event+"|"+string(f.Data))
		return nil, true
	}}
	req, err := http.NewRequest(http.MethodPost, "http://example.invalid", nil)
	if err != nil {
		t.Fatal(err)
	}
	for range Stream(context.Background(), &http.Client{Transport: bodyTransport(body.String())}, req, c, Options{}) {
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	if len(buffers) > len(got)/2 {
		t.Fatalf("%d frames used %d buffers; frame data is not being reused", len(got), len(buffers))
	}
}

func BenchmarkStreamFrames(b *testing.B) {
	body := strings.Repeat("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"hello there, a typical delta\"}}\n\n", 1000) + "data: stop\n\n"
	client := &http.Client{Transport: bodyTransport(body)}
	c := conv{convert: func(f Frame) ([]api.Event, bool) {
		return nil, string(f.Data) != "stop"
	}}
	b.ReportAllocs()
	for b.Loop() {
		req, err := http.NewRequest(http.MethodPost, "http://example.invalid", nil)
		if err != nil {
			b.Fatal(err)
		}
		for range Stream(context.Background(), client, req, c, Options{}) {
		}
	}
}
