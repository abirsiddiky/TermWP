package core

import (
	"reflect"
	"testing"
)

func TestRollbackStackLIFO(t *testing.T) {
	stack := NewRollbackStack()
	var executed []string

	stack.Push("step1", func() error {
		executed = append(executed, "undo1")
		return nil
	})
	stack.Push("step2", func() error {
		executed = append(executed, "undo2")
		return nil
	})
	stack.Push("step3", func() error {
		executed = append(executed, "undo3")
		return nil
	})

	stack.Execute(nil)

	expected := []string{"undo3", "undo2", "undo1"}
	if !reflect.DeepEqual(executed, expected) {
		t.Errorf("expected LIFO rollback order %v, got %v", expected, executed)
	}
}

func TestRollbackStackDiscard(t *testing.T) {
	stack := NewRollbackStack()
	var executed []string

	stack.Push("step1", func() error {
		executed = append(executed, "undo1")
		return nil
	})

	stack.Discard()
	stack.Execute(nil)

	if len(executed) != 0 {
		t.Errorf("expected no steps executed after discard, got %v", executed)
	}
}
