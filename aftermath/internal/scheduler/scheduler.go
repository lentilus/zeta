package scheduler

import (
	"log"
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
				log.Printf("Executing %s task...\n", task.Name)
				task.Execute()
				s.wg.Done() // Mark the task as completed
			case <-s.stopChan:
				// Stop signal received, drain the taskQueue and exit
				for task := range s.taskQueue {
					log.Printf("Draining task: %s\n", task.Name)
					task.Execute()
					s.wg.Done()
				}
				return
			}
		}
	}()
}

// SchedulePeriodicTask periodically runs low-priority tasks without blocking
func (s *Scheduler) SchedulePeriodicTask(interval time.Duration, lowTask Task) {
	ticker := time.NewTicker(interval)

	// Run the task on startup in a non-blocking manner
	go func() {
		s.lowPriorityLock.Lock()
		defer s.lowPriorityLock.Unlock()
		lowTask.Execute()
	}()

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				go func() {
					s.lowPriorityLock.Lock()
					defer s.lowPriorityLock.Unlock()

					select {
					case s.taskQueue <- lowTask:
						log.Printf("Scheduled %s.", lowTask.Name)
						s.wg.Add(1) // Add to wait group for the low-priority task
					default:
						log.Printf("Skipped scheduling %s. Queue is full.", lowTask.Name)
					}
				}()
			case <-s.stopChan:
				// Stop scheduling periodic tasks
				return
			}
		}
	}()
}

// ScheduleHighPriorityTask runs a high-priority task asap
func (s *Scheduler) ScheduleHighPriorityTask(task Task) {
	s.wg.Add(1) // Add to wait group for the high-priority task
	s.taskQueue <- task
}

// StopScheduler waits for all tasks to complete and stops the scheduler
func (s *Scheduler) StopScheduler() {
	log.Println("Stopping scheduler.")
	close(s.stopChan)  // Signal the scheduler to stop
	close(s.taskQueue) // Close the task queue to prevent further submissions
	s.wg.Wait()        // Wait for all tasks to complete
	log.Println("Scheduler stopped.")
}
