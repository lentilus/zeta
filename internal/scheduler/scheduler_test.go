package scheduler_test

import (
	scheduler "aftermath/internal/index"
	"testing"
	"time"
)

func TestSchedulerStop(t *testing.T) {
	// Create a new Scheduler with a buffer size of 10
	s := scheduler.NewScheduler(10)

	// Channels to track task execution
	taskExecuted := make(chan string, 10)

	// Define a task that signals execution
	testTask := scheduler.Task{
		Name: "TestTask",
		Execute: func() {
			time.Sleep(100 * time.Millisecond) // Simulate work
			taskExecuted <- "TestTask executed"
		},
	}

	// Start the scheduler
	s.RunScheduler()

	// Schedule a few tasks
	for i := 0; i < 5; i++ {
		s.ScheduleHighPriorityTask(testTask)
	}

	// Stop the scheduler
	go func() {
		time.Sleep(500 * time.Millisecond) // Let some tasks execute
		s.StopScheduler()
	}()

	// Wait for tasks to be executed
	executedCount := 0
	timeout := time.After(2 * time.Second)

	for {
		select {
		case msg := <-taskExecuted:
			t.Log(msg)
			executedCount++
		case <-timeout:
			if executedCount < 5 {
				t.Fatalf("Expected all tasks to execute, but only %d completed", executedCount)
			}
			return
		}
	}
}
