package cron

import (
	"context"
	"testing"
	"time"

	"go_backend/internal/pkg/locker"
	"go.uber.org/zap"
)

func TestTaskScheduler_JobContext(t *testing.T) {
	s := NewTaskScheduler(nil, nil, nil, nil, nil, nil, locker.NewTaskLocker(nil, zap.NewNop()), zap.NewNop())

	ctx, cancel := s.jobContext(50 * time.Millisecond)
	defer cancel()

	select {
	case <-ctx.Done():
		t.Fatal("Context should not be done immediately")
	default:
	}

	time.Sleep(70 * time.Millisecond)

	select {
	case <-ctx.Done():
		if ctx.Err() != context.DeadlineExceeded {
			t.Fatalf("Expected DeadlineExceeded, got %v", ctx.Err())
		}
	default:
		t.Fatal("Context should be done after timeout")
	}
}

func TestTaskScheduler_StopWait_NormalCleanup(t *testing.T) {
	s := NewTaskScheduler(nil, nil, nil, nil, nil, nil, locker.NewTaskLocker(nil, zap.NewNop()), zap.NewNop())

	jobCtx, cancel := s.jobContext(10 * time.Second)
	defer cancel()

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer stopCancel()

	if err := s.StopWait(stopCtx); err != nil {
		t.Fatalf("StopWait failed: %v", err)
	}

	// Since scheduler stopped, lifecycleCtx should be cancelled
	select {
	case <-jobCtx.Done():
		// Expected!
	case <-time.After(200 * time.Millisecond):
		t.Fatal("jobCtx should have been cancelled when scheduler stopped")
	}
}

func TestTaskScheduler_StopWait_InFlightTimeoutCancellation(t *testing.T) {
	s := NewTaskScheduler(nil, nil, nil, nil, nil, nil, locker.NewTaskLocker(nil, zap.NewNop()), zap.NewNop())

	jobStarted := make(chan struct{})
	jobCancelled := make(chan struct{})

	// Register a slow job directly in s.cron
	_, err := s.cron.AddFunc("@every 1s", func() {
		close(jobStarted)
		jobCtx, cancel := s.jobContext(10 * time.Minute)
		defer cancel()

		<-jobCtx.Done()
		close(jobCancelled)
	})
	if err != nil {
		t.Fatalf("AddFunc failed: %v", err)
	}

	s.cron.Start()

	// Wait for job to start
	select {
	case <-jobStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("Job did not start in time")
	}

	// StopWait with a tiny timeout (50ms) to trigger timeout branch
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer stopCancel()

	err = s.StopWait(stopCtx)
	if err != context.DeadlineExceeded {
		t.Fatalf("Expected DeadlineExceeded from StopWait, got %v", err)
	}

	// The in-flight job should have its jobCtx cancelled by s.cancelLifecycle()!
	select {
	case <-jobCancelled:
		// Expected! The in-flight job unblocked because jobCtx was cancelled
	case <-time.After(1 * time.Second):
		t.Fatal("In-flight job was not cancelled after StopWait timed out")
	}
}
