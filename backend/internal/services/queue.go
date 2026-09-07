package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	QueueRuleEvaluation = "thureos:queue:rule_eval"
	QueueDead           = "thureos:queue:dead"
	MaxRetries          = 3
)

// Tipos de job sobre la misma cola: el Kind vacío es evaluación de reglas
// (los jobs históricos no traen Kind), JobKindNotify entrega notificaciones.
const (
	JobKindEval   = "eval"
	JobKindNotify = "notify"
)

type EvalJob struct {
	Kind string `json:"kind,omitempty"`
	// Evaluación de reglas
	MonitorID string `json:"monitorId,omitempty"`
	// Notificación de red flag nueva
	Notify *NotifyPayload `json:"notify,omitempty"`

	Retries   int   `json:"retries"`
	CreatedAt int64 `json:"createdAt"`
}

type JobQueue struct {
	redis *redis.Client
}

func NewJobQueue(redisClient *redis.Client) *JobQueue {
	return &JobQueue{redis: redisClient}
}

func (q *JobQueue) Enqueue(ctx context.Context, job EvalJob) error {
	if q.redis == nil {
		return fmt.Errorf("redis not available")
	}
	job.CreatedAt = time.Now().UnixMilli()

	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshaling job: %w", err)
	}

	return q.redis.LPush(ctx, QueueRuleEvaluation, data).Err()
}

// Dequeue blocks until a job is available or timeout expires.
// Returns nil job on timeout (not an error).
func (q *JobQueue) Dequeue(ctx context.Context, timeout time.Duration) (*EvalJob, error) {
	if q.redis == nil {
		return nil, fmt.Errorf("redis not available")
	}

	result, err := q.redis.BRPop(ctx, timeout, QueueRuleEvaluation).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var job EvalJob
	if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
		return nil, fmt.Errorf("unmarshaling job: %w", err)
	}

	return &job, nil
}

func (q *JobQueue) SendToDead(ctx context.Context, job EvalJob, reason string) error {
	dead := map[string]interface{}{
		"job":    job,
		"reason": reason,
		"diedAt": time.Now().UnixMilli(),
	}
	data, _ := json.Marshal(dead)
	return q.redis.LPush(ctx, QueueDead, data).Err()
}

func (q *JobQueue) QueueLength(ctx context.Context) (int64, error) {
	if q.redis == nil {
		return 0, nil
	}
	return q.redis.LLen(ctx, QueueRuleEvaluation).Result()
}
