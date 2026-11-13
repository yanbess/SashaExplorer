package undo

import (
	"errors"
	"testing"
)

func TestUndoRedoOrderAndFailures(t *testing.T) {
	var log []string
	var states []State
	j := New(func(s State) { states = append(states, s) })
	step := func(name string, fail *bool) Step {
		return Step{
			Label: name,
			Undo: func() error {
				if fail != nil && *fail {
					return errors.New("blocked")
				}
				log = append(log, "undo "+name)
				return nil
			},
			Redo: func() error { log = append(log, "redo "+name); return nil },
		}
	}
	blocked := true
	j.Add(step("a", nil))
	j.Add(step("b", &blocked))
	if _, err := j.Undo(); err == nil {
		t.Fatal("failed undo reported success")
	}
	if s := j.State(); s.UndoLabel != "b" || s.CanRedo {
		t.Fatalf("failed step moved: %+v", s)
	}
	blocked = false
	j.Undo()
	j.Undo()
	if _, err := j.Undo(); !errors.Is(err, ErrNothing) {
		t.Errorf("empty undo: %v", err)
	}
	j.Redo()
	if s := j.State(); s.UndoLabel != "a" || s.RedoLabel != "b" {
		t.Errorf("state %+v", s)
	}
	j.Add(step("c", nil))
	if j.State().CanRedo {
		t.Error("new step kept redo")
	}
	want := []string{"undo b", "undo a", "redo a"}
	if len(log) != 3 || log[0] != want[0] || log[1] != want[1] || log[2] != want[2] {
		t.Errorf("log %v", log)
	}
	if len(states) == 0 {
		t.Error("no reports")
	}
}
