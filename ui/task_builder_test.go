package ui_test

import (
	"fmt"
	"testing"

	"github.com/stacc/stacc-cli-library/ui"
)

// TestTaskBuilderRun calls ui.TaskBuilder.Run with an empty function,
// checking for errors
func TestTaskBuilderRun(t *testing.T) {
	didRunFunction := false
	_, err := ui.TaskBuilder{Silence: true}.AddTask("test", nil, func(context map[string]interface{}) error {
		didRunFunction = true
		return nil
	}).Run()
	if err != nil {
		t.Error("TestTaskBuilderRun expected TaskBuilder.Run to not error when running test task")
	}

	if !didRunFunction {
		t.Error("TestTaskBuilderRun expected TaskBuilder.AddTask function to run to completion")
	}
}

// TestTaskBuilderContext calls ui.TaskBuilder.AddTask with functions that sets context,
// checking that the returned context was set correctly
func TestTaskBuilderContext(t *testing.T) {
	ctx, err := ui.TaskBuilder{Silence: true}.AddTask("task1", nil, func(context map[string]interface{}) error {
		context["task1"] = "foo"
		return nil
	}).AddTask("task2", nil, func(context map[string]interface{}) error {
		if val, ok := context["task1"]; !ok || val != "foo" {
			t.Error("Context was not set when running task")
		}
		context["task2"] = "bar"
		return nil
	}).Run()

	if err != nil {
		t.Error("TestTaskBuilderContext expected TaskBuilder.Run to not error when running test task")
	}

	if val, ok := ctx["task1"]; !ok || val != "foo" {
		t.Error("TestTaskBuilderContext expected context map to include key value pair task1:foo")
	}

	if val, ok := ctx["task2"]; !ok || val != "bar" {
		t.Error("TestTaskBuilderContext expected context map to include key value pair task2:bar")
	}
}

// TestTaskBuilderShouldFail calls ui.TaskBuilder.AddTask with a function that returns an error,
// checking that the ui.TaskBuilder.Run function also returns an error
func TestTaskBuilderShouldFail(t *testing.T) {
	_, err := ui.TaskBuilder{Silence: true}.AddTask("task1", nil, func(context map[string]interface{}) error {
		return nil
	}).AddTask("task2", nil, func(context map[string]interface{}) error {
		return fmt.Errorf("error meant for testing")
	}).AddTask("task3", nil, func(context map[string]interface{}) error {
		return nil
	}).Run()

	if err == nil {
		t.Errorf("TestTaskBuilderShouldFail expected TaskBuilder.Run to error when task errors")
	}
}
