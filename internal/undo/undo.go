// Package undo keeps the file operations of this program, so they can be undone
// and redone: renames, new items, moves, copies, and deletions to the Recycle Bin.
package undo

import (
	"errors"
	"sync"
)

// MaxSteps limits how far back undo goes.
const MaxSteps = 50

// Step is one operation that can be undone and done again.
type Step struct {
	// Label describes the operation, such as "rename".
	Label string
	Undo  func() error
	Redo  func() error
}

// State tells the buttons what undo and redo would do.
type State struct {
	CanUndo   bool   `json:"canUndo"`
	CanRedo   bool   `json:"canRedo"`
	UndoLabel string `json:"undoLabel"`
	RedoLabel string `json:"redoLabel"`
}

// Journal is the list of steps. One step runs at a time.
type Journal struct {
	mu      sync.Mutex
	run     sync.Mutex
	done    []Step
	undone  []Step
	changed func(State)
}

// New creates a journal that reports changes to changed.
func New(changed func(State)) *Journal {
	return &Journal{changed: changed}
}

// ErrNothing means there is no step to undo or redo.
var ErrNothing = errors.New("there is nothing to undo")

// Add records a step that just happened. It ends the redo list, as in any editor.
func (j *Journal) Add(step Step) {
	j.mu.Lock()
	j.done = append(j.done, step)
	if len(j.done) > MaxSteps {
		j.done = j.done[len(j.done)-MaxSteps:]
	}
	j.undone = nil
	state := j.stateLocked()
	j.mu.Unlock()
	j.report(state)
}

// Undo reverts the last step. A step that fails stays where it was, so the user
// can fix the cause and try again.
func (j *Journal) Undo() (string, error) {
	return j.move(true)
}

// Redo does the last undone step again.
func (j *Journal) Redo() (string, error) {
	return j.move(false)
}

func (j *Journal) move(back bool) (string, error) {
	j.run.Lock()
	defer j.run.Unlock()

	j.mu.Lock()
	from := &j.done
	if !back {
		from = &j.undone
	}
	if len(*from) == 0 {
		j.mu.Unlock()
		if back {
			return "", ErrNothing
		}
		return "", errors.New("there is nothing to redo")
	}
	step := (*from)[len(*from)-1]
	j.mu.Unlock()

	action := step.Redo
	if back {
		action = step.Undo
	}
	if err := action(); err != nil {
		return step.Label, err
	}

	j.mu.Lock()
	if back {
		j.done = j.done[:len(j.done)-1]
		j.undone = append(j.undone, step)
	} else {
		j.undone = j.undone[:len(j.undone)-1]
		j.done = append(j.done, step)
	}
	state := j.stateLocked()
	j.mu.Unlock()
	j.report(state)
	return step.Label, nil
}

// State returns what undo and redo would do now.
func (j *Journal) State() State {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.stateLocked()
}

func (j *Journal) stateLocked() State {
	state := State{CanUndo: len(j.done) > 0, CanRedo: len(j.undone) > 0}
	if state.CanUndo {
		state.UndoLabel = j.done[len(j.done)-1].Label
	}
	if state.CanRedo {
		state.RedoLabel = j.undone[len(j.undone)-1].Label
	}
	return state
}

func (j *Journal) report(state State) {
	if j.changed != nil {
		j.changed(state)
	}
}
