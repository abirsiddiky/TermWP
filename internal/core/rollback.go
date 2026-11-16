package core

import (
	"fmt"
	"termwp/internal/ui"
)

// RollbackStep defines an individual step to reverse on failure.
type RollbackStep struct {
	Name string
	Undo func() error
}

// RollbackStack tracks transactional steps and executes reversals in LIFO order upon failure.
type RollbackStack struct {
	steps []RollbackStep
}

// NewRollbackStack initializes an empty RollbackStack.
func NewRollbackStack() *RollbackStack {
	return &RollbackStack{
		steps: make([]RollbackStep, 0),
	}
}

// Push registers a compensation action.
func (rs *RollbackStack) Push(name string, undo func() error) {
	rs.steps = append(rs.steps, RollbackStep{Name: name, Undo: undo})
}

// Execute triggers all registered undo actions in reverse order (LIFO).
func (rs *RollbackStack) Execute(tracker *ui.StepTracker) {
	if len(rs.steps) == 0 {
		return
	}

	if tracker != nil {
		tracker.Warn("Rolling back changes due to failure...")
	}

	for i := len(rs.steps) - 1; i >= 0; i-- {
		step := rs.steps[i]
		if err := step.Undo(); err != nil {
			if tracker != nil {
				tracker.Fail(fmt.Sprintf("Rollback step failed (%s): %v", step.Name, err))
			}
		} else if tracker != nil {
			tracker.Info(fmt.Sprintf("Reversed: %s", step.Name))
		}
	}
}

// Discard clears all rollback steps when the transaction succeeds.
func (rs *RollbackStack) Discard() {
	rs.steps = nil
}
