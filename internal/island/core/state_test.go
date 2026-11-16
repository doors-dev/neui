package core

import (
	"context"
	"errors"
	"html"
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

type capRun struct {
	Run
	cap int
}

func (r capRun) NotificationsCap() int { return r.cap }

func TestLastActionCarriesTheDrawnOutputUpToTheNotificationsCap(t *testing.T) {
	full := `{"id":3,"output":{"name":"Ann"}}`
	for name, tc := range map[string]struct {
		cap  int
		want string
	}{
		"at the cap":   {len(html.EscapeString(full)), full},
		"over the cap": {len(html.EscapeString(full)) - 1, `{"id":3,"output_omitted":true}`},
	} {
		s := &state{run: capRun{cap: tc.cap}}
		if got := s.LastAction(); got != "" {
			t.Fatalf("%s: before any action the last action is %s, want none", name, got)
		}
		s.stash = &submit{id: 3, data: map[string]string{"name": "Ann"}}
		if r, err := s.collect(); err != nil || r.ID != 3 {
			t.Fatalf("%s: collect returned %+v, %v, want the stashed action", name, r, err)
		}
		if got := s.LastAction(); got != tc.want {
			t.Fatalf("%s: the last action is %s, want %s", name, got, tc.want)
		}
		s.ForgetAction()
		if got := s.LastAction(); got != "" {
			t.Fatalf("%s: a forgotten action is still %s", name, got)
		}
	}
}

func TestDrawForgetsTheLastAction(t *testing.T) {
	s := &state{run: &turnRun{}, action: &Result{ID: 3, Output: "saved"}}
	if _, err := s.toolDraw(context.Background(), 0); err == nil {
		t.Fatal("toolDraw ignored the GC error")
	}
	if got := s.LastAction(); got != "" {
		t.Fatalf("after the script drew again the last action is %s, want none", got)
	}
}
