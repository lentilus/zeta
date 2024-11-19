package scheduler

import (
	"fmt"
	"sync"
	"time"
)

type Task struct {
	Name    string
	Execute func() error
}

type Scheduler struct {
	taskQueue       chan Task
	lowPriorityLock sync.Mutex
	stopChan        chan struct{}
	wg              sync.WaitGroup
}

// NewScheduler creates a new Scheduler with the specified queue size
func NewScheduler(queueSize int) *Scheduler {
	return &Scheduler{
		taskQueue: make(chan Task, queueSize),
		stopChan:  make(chan struct{}),
	}
}

// RunScheduler starts the scheduler loop
func (s *Scheduler) RunScheduler() {
	go func() {
		for {
			select {
			case task, ok := <-s.taskQueue:
				if !ok {
					// Channel closed, exit the loop
					return
				}
				fmt.Printf("Executing %s task...\n", task.Name)
				task.Execute()
				s.wg.Done() // Mark the task as completed
			case <-s.stopChan:
				// Stop signal received, drain the taskQueue and exit
				for task := range s.taskQueue {
					fmt.Printf("Draining task: %s\n", task.Name)
					task.Execute()
					s.wg.Done()
				}
				return
			}
		}
	}()
}

// SchedulePeriodicTask periodically runs low-priority tasks
func (s *Scheduler) SchedulePeriodicTask(interval time.Duration, lowTask Task) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run the task on startup
	s.lowPriorityLock.Lock()
	lowTask.Execute()
	s.wg.Add(1) // Add to wait group for the low-priority task
	s.lowPriorityLock.Unlock()

	for {
		select {
		case <-ticker.C:
			// Ensure low-priority tasks don't interfere with high-priority task handling
			s.lowPriorityLock.Lock()
			select {
			case s.taskQueue <- lowTask:
				fmt.Println("Scheduled low-priority task.")
				s.wg.Add(1) // Add to wait group for the low-priority task
			default:
				fmt.Println("Skipped scheduling low-priority task due to full queue.")
			}
			s.lowPriorityLock.Unlock()
		case <-s.stopChan:
			// Stop scheduling periodic tasks
			return
		}
	}
}

// ScheduleHighPriorityTask runs a high-priority task asap
func (s *Scheduler) ScheduleHighPriorityTask(task Task) {
	s.wg.Add(1) // Add to wait group for the high-priority task
	s.taskQueue <- task
}

// StopScheduler waits for all tasks to complete and stops the scheduler
func (s *Scheduler) StopScheduler() {
	fmt.Println("Stopping scheduler...")
	close(s.stopChan)  // Signal the scheduler to stop
	close(s.taskQueue) // Close the task queue to prevent further submissions
	s.wg.Wait()        // Wait for all tasks to complete
	fmt.Println("Scheduler stopped.")
}
