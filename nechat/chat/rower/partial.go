package rower

import (
	"time"

	"github.com/doors-dev/gox"
	"neui.dev/nechat/chat/rower/marksman"
)

const partialInterval = 3 * time.Second

func newPartialRower(m map[int]string) *rower[partialText] {
	pm := make(map[int]streamer[partialText], len(m))
	for index, content := range m {
		pm[index] = partialStreamer{marksman.New(content, partialInterval, nil)}
	}
	return newRower(pm, func(pt partialText) streamer[partialText] {
		return partialStreamer{marksman.New(pt.text, partialInterval, nil)}
	})
}

type partialText struct {
	part int
	text string
}

func (p partialText) At() int {
	return p.part
}

type partialStreamer struct {
	marksman.Marksman
}

var _ streamer[partialText] = partialStreamer{}

func (t partialStreamer) Main() gox.Elem {
	return gox.Elem(func(cur gox.Cursor) error {
		if err := cur.Init("div"); err != nil {
			return err
		}
		if err := cur.Set("class", "nechat-col nechat-prose"); err != nil {
			return err
		}
		if err := cur.Submit(); err != nil {
			return err
		}
		if err := cur.Comp(t.Marksman); err != nil {
			return err
		}
		return cur.Close()
	})
}

func (t partialStreamer) Append(p partialText) {
	t.Marksman.Append(p.text)
}
