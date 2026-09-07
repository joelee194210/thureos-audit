package services

import (
	"context"
	"log"
	"time"

	"github.com/thureos/compliance/internal/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Worker struct {
	queue       *JobQueue
	ruleEngine  *RuleEngine
	monitorRepo *repository.MonitorRepository
	notifier    *NotificationService
	concurrency int
}

func NewWorker(
	queue *JobQueue,
	ruleEngine *RuleEngine,
	monitorRepo *repository.MonitorRepository,
	concurrency int,
	notifier ...*NotificationService,
) *Worker {
	if concurrency < 1 {
		concurrency = 2
	}
	w := &Worker{
		queue:       queue,
		ruleEngine:  ruleEngine,
		monitorRepo: monitorRepo,
		concurrency: concurrency,
	}
	if len(notifier) > 0 {
		w.notifier = notifier[0]
	}
	return w
}

// Start launches worker goroutines that process the queue.
// Blocks until ctx is cancelled.
func (w *Worker) Start(ctx context.Context) {
	log.Printf("Starting %d rule evaluation workers", w.concurrency)

	for i := range w.concurrency {
		go w.loop(ctx, i)
	}

	<-ctx.Done()
	log.Println("Workers shutting down")
}

func (w *Worker) loop(ctx context.Context, workerID int) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		job, err := w.queue.Dequeue(ctx, 5*time.Second)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("Worker %d: dequeue error: %v", workerID, err)
			time.Sleep(time.Second)
			continue
		}

		if job == nil {
			continue
		}

		w.process(ctx, workerID, job)
	}
}

func (w *Worker) process(ctx context.Context, workerID int, job *EvalJob) {
	if job.Kind == JobKindNotify {
		w.processNotify(ctx, workerID, job)
		return
	}

	monitorID, err := primitive.ObjectIDFromHex(job.MonitorID)
	if err != nil {
		log.Printf("Worker %d: invalid monitor ID %s", workerID, job.MonitorID)
		return
	}

	monitor, err := w.monitorRepo.FindByID(ctx, monitorID)
	if err != nil {
		log.Printf("Worker %d: monitor not found %s: %v", workerID, job.MonitorID, err)
		return
	}

	redFlags, err := w.ruleEngine.EvaluateRules(ctx, monitor)
	if err != nil {
		log.Printf("Worker %d: evaluation failed for %s: %v", workerID, monitor.Name, err)

		job.Retries++
		if job.Retries >= MaxRetries {
			if sendErr := w.queue.SendToDead(ctx, *job, err.Error()); sendErr != nil {
				log.Printf("Worker %d: enviando job a la cola muerta: %v", workerID, sendErr)
			}
			log.Printf("Worker %d: job sent to dead queue after %d retries", workerID, job.Retries)
			return
		}

		// Re-enqueue for retry
		if enqErr := w.queue.Enqueue(ctx, *job); enqErr != nil {
			log.Printf("Worker %d: re-enqueue failed: %v", workerID, enqErr)
		}
		return
	}

	if len(redFlags) > 0 {
		log.Printf("Worker %d: %d red flags triggered for monitor %s", workerID, len(redFlags), monitor.Name)
	}
}

// processNotify entrega una notificación con la misma política de
// reintentos que la evaluación: MaxRetries y luego cola muerta.
func (w *Worker) processNotify(ctx context.Context, workerID int, job *EvalJob) {
	if w.notifier == nil || job.Notify == nil {
		return
	}
	if err := w.notifier.ProcessNotification(ctx, job.Notify); err != nil {
		log.Printf("Worker %d: notification for red flag %s failed: %v", workerID, job.Notify.RedFlagID, err)

		job.Retries++
		if job.Retries >= MaxRetries {
			if sendErr := w.queue.SendToDead(ctx, *job, err.Error()); sendErr != nil {
				log.Printf("Worker %d: enviando notificación a la cola muerta: %v", workerID, sendErr)
			}
			return
		}
		if enqErr := w.queue.Enqueue(ctx, *job); enqErr != nil {
			log.Printf("Worker %d: re-enqueue de notificación falló: %v", workerID, enqErr)
		}
	}
}
