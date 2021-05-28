package ui

import (
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/tj/go-spin"
)

type TaskBuilder struct {
	Silence bool // silences all console output

	tasks []task
}

type task struct {
	title    string
	skipFunc func(context map[string]interface{}) (bool, string)
	runFunc  func(ctx map[string]interface{}) error
	stop     chan int
}

func NewTaskBuilder() TaskBuilder {
	return TaskBuilder{Silence: false}
}

func (tb TaskBuilder) AddTask(title string, skipFunc func(context map[string]interface{}) (bool, string), runFunc func(context map[string]interface{}) error) TaskBuilder {
	task := task{
		title:    title,
		skipFunc: skipFunc,
		runFunc:  runFunc,
		stop:     make(chan int, 1),
	}

	tb.tasks = append(tb.tasks, task)

	return tb
}

func (tb TaskBuilder) Run() (map[string]interface{}, error) {
	ctx := make(map[string]interface{})
	for _, task := range tb.tasks {
		err := task.run(ctx, tb.Silence)
		if err != nil {
			return nil, err
		}
	}

	return ctx, nil
}

// FIXME: There's a chance to get a spinner on a completed task
func (t task) run(ctx map[string]interface{}, silence bool) error {
	if t.runFunc == nil {
		return fmt.Errorf("task: runFunc is nil")
	}

	if t.skipFunc != nil {
		shouldSkip, message := t.skipFunc(ctx)
		if shouldSkip {
			if !silence {
				t.skip(message)
			}
			return nil
		}
	}

	spinner := spin.New()
	yellow := color.New(color.FgYellow)
	go func() {
		for {
			select {
			case <-t.stop:
				return
			default:
				if !silence {
					yellow.Printf("\r  %s ", spinner.Next())
					fmt.Print(t.title)
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	err := t.runFunc(ctx)
	t.stop <- 0
	if err != nil {
		if !silence {
			t.fail(err)
		}
		return err
	}

	if !silence {
		t.complete()
	}
	return nil
}

func (t task) complete() {
	color.New(color.FgGreen).Print("\r  ✔ ")
	fmt.Println(t.title)
}

func (t task) fail(err error) {
	color.New(color.FgRed).Print("\r  ✘ ")
	fmt.Println(t.title)
	color.New(color.FgHiBlack).Print("    ↪ %s\n", err.Error())
}

func (t task) skip(message string) {
	color.New(color.FgYellow).Print("\r  ⮎ ")
	fmt.Print(t.title)
	grey := color.New(color.FgHiBlack)
	grey.Print(" [skipped]")
	if message != "" {
		grey.Printf("    ↪ %s\n", message)
	}
}
