package core

import (
	"context"
	"errors"
	"slices"
	"testing"
)

type turnRun struct {
	Run
	events []string
}

func (r *turnRun) PauseTurn()  { r.events = append(r.events, "pause") }
func (r *turnRun) ResumeTurn() { r.events = append(r.events, "resume") }
func (r *turnRun) GC() error {
	r.events = append(r.events, "gc")
	return errors.New("gc failed")
}

func TestDrawRunsPendingJobsUnderTheTurnDeadline(t *testing.T) {
	applied := make(chan struct{})
	close(applied)
	for name, tc := range map[string]struct {
		applied chan struct{}
		want    []string
	}{
		"first draw": {nil, []string{"gc"}},
		"later draw": {applied, []string{"pause", "resume", "gc"}},
	} {
		run := &turnRun{}
		s := &state{run: run, applied: tc.applied}
		if _, err := s.toolDraw(context.Background(), 0); err == nil {
			t.Fatalf("%s: toolDraw ignored the GC error", name)
		}
		if !slices.Equal(run.events, tc.want) {
			t.Fatalf("%s: events %v, want %v", name, run.events, tc.want)
		}
	}
}
