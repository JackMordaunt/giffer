package main

import (
	"fmt"
	"sync"

	"github.com/hashicorp/go-multierror"
)

// Task is an independent unit of work.
type Task struct {
	Name     string
	Op       func() error
	Requires []string
}

// FanOut executes all the independant tasks in parallel and waits for them to
// finish.
type FanOut []Task

// Run all the tasks.
func (tasks FanOut) Run() (err error) {
	var (
		failures = make(chan error)
		wg       = &sync.WaitGroup{}
		// index contains all the Tasks and allows us to resolve a Task
		// object from it's name alone.
		// This is used for building the graph.
		index = map[string]Task{}
		// Each chain is a serialised list of tasks to execute.
		// Chains can be executed independently because they do not
		// share dependencies - thus parallelism.
		chains [][]Task
	)
	for _, t := range tasks {
		index[t.Name] = t
	}
	g := &Graph{}
	for _, t := range tasks {
		g.Append(taskNode{Task: t, Index: index})
	}
	resolved := g.Resolve()
	// Since the resolved slice is an ordered list of tasks, we can slice it
	// into independent chains delimited by tasks that have no dependencies.
	for _, n := range resolved {
		if len(n.Requires()) > 0 {
			// Append to the current chain.
			current := len(chains) - 1
			chains[current] = append(chains[current], index[n.ID()])
		} else {
			// Start a new chain.
			chains = append(chains, []Task{index[n.ID()]})
		}
	}
	wg.Add(len(chains))
	for _, chain := range chains {
		chain := chain
		go func() {
			defer wg.Done()
			for _, t := range chain {
				fmt.Printf("run: %v\n", t.Name)
				if err := t.Op(); err != nil {
					failures <- TaskError{Task: t, Err: err}
					break
				}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(failures)
	}()
	for failure := range failures {
		err = multierror.Append(err, failure)
	}
	return err
}

// TaskError associates an error with the task that produced it.
type TaskError struct {
	Task Task
	Err  error
}

func (err TaskError) Error() string {
	return fmt.Sprintf("task %q: %v", err.Task.Name, err.Err)
}
