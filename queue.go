package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/goravel/framework/contracts/config"
	"github.com/goravel/framework/contracts/foundation"
	contractsqueue "github.com/goravel/framework/contracts/queue"
	"github.com/goravel/framework/errors"
	"github.com/goravel/framework/support/carbon"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
)

type Task struct {
	Job
	UUID  string `json:"uuid"`
	Chain []Job  `json:"chain"`
}

type Job struct {
	Signature string               `json:"signature"`
	Args      []contractsqueue.Arg `json:"args"`
	Delay     *time.Time           `json:"delay"`
}

var _ contractsqueue.Driver = &Queue{}

// maxMigrateJobsPerPop caps how many jobs a single Pop call migrates from
// the delayed/reserved sets back to the ready queue. This keeps a single
// Pop from blocking on a huge backlog; remaining jobs are migrated by the
// next Pop call.
const maxMigrateJobsPerPop = 100

type Queue struct {
	ctx        context.Context
	client     redis.UniversalClient
	jobStorer  contractsqueue.JobStorer
	json       foundation.Json
	queueKey   *QueueKey
	retryAfter int
}

func NewQueue(ctx context.Context, config config.Config, queue contractsqueue.Queue, json foundation.Json, connection string) (*Queue, error) {
	clientConnection := config.GetString(fmt.Sprintf("queue.connections.%s.connection", connection), "default")
	client, err := GetClient(config, clientConnection)
	if err != nil {
		return nil, fmt.Errorf("failed to init redis client: %w", err)
	}

	return &Queue{
		ctx:        ctx,
		client:     client,
		jobStorer:  queue.JobStorer(),
		json:       json,
		queueKey:   NewQueueKey(config.GetString("app.name", "goravel"), connection),
		retryAfter: config.GetInt(fmt.Sprintf("queue.connections.%s.retry_after", connection), 60),
	}, nil
}

func (r *Queue) Driver() string {
	return contractsqueue.DriverCustom
}

func (r *Queue) Later(delay time.Time, task contractsqueue.Task, queue string) error {
	// The main job delay is set in redis, so we need to set it to zero to avoid double delay.
	task.Delay = time.Time{}
	payload, err := taskToJobRecordJson(task, r.json)
	if err != nil {
		return err
	}

	return r.client.ZAdd(r.ctx, r.queueKey.Delayed(queue), redis.Z{
		Score:  float64(delay.Unix()),
		Member: payload,
	}).Err()
}

func (r *Queue) Pop(queue string) (contractsqueue.ReservedJob, error) {
	queueKey := r.queueKey.Queue(queue)

	if err := r.migrateDelayedJobs(queue); err != nil {
		return nil, err
	}

	if err := r.migrateExpiredReservedJobs(queue); err != nil {
		return nil, err
	}

	result, err := r.client.LPop(r.ctx, queueKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, errors.QueueDriverNoJobFound.Args(queueKey)
		}
		return nil, err
	}

	jobRecord, err := jsonToJobRecord(result, r.json)
	if err != nil {
		return nil, err
	}

	return NewReservedJob(r.ctx, r.client, jobRecord, r.jobStorer, r.json, r.queueKey.Reserved(queue), r.queueKey.Delayed(queue))
}

func (r *Queue) Push(task contractsqueue.Task, queue string) error {
	if !task.Delay.IsZero() {
		return r.Later(task.Delay, task, queue)
	}

	payload, err := taskToJobRecordJson(task, r.json)
	if err != nil {
		return err
	}

	return r.client.RPush(r.ctx, r.queueKey.Queue(queue), payload).Err()
}

func (r *Queue) migrateDelayedJobs(queue string) error {
	queueKey := r.queueKey.Queue(queue)
	delayQueueKey := r.queueKey.Delayed(queue)
	now := float64(carbon.Now().Timestamp())

	// Lua script: atomically check score, remove from ZSET, and push to list only if ready
	script := `
		local delayed = KEYS[1]
		local queue = KEYS[2]
		local now = tonumber(ARGV[1])
		local job = redis.call('ZRANGE', delayed, 0, 0, 'WITHSCORES')
		if #job == 0 then
			return 0
		end
		local score = tonumber(job[2])
		if score > now then
			return 0
		end
		redis.call('ZREM', delayed, job[1])
		redis.call('RPUSH', queue, job[1])
		return 1
	`

	// Cap the number of jobs migrated per call so a single Pop does not
	// block on a huge backlog; the next Pop call continues the migration.
	for i := 0; i < maxMigrateJobsPerPop; i++ {
		result, err := r.client.Eval(r.ctx, script, []string{delayQueueKey, queueKey}, now).Result()
		if err != nil {
			return err
		}
		if cast.ToInt64(result) == 0 {
			break
		}
	}

	return nil
}

// migrateExpiredReservedJobs recovers jobs left reserved by a crashed
// worker. Any reservation with score ≤ (now - retryAfter) is moved
// back to the ready queue. retry_after must exceed the maximum job
// runtime to avoid double-processing (same contract as Laravel).
func (r *Queue) migrateExpiredReservedJobs(queue string) error {
	reservedKey := r.queueKey.Reserved(queue)
	queueKey := r.queueKey.Queue(queue)
	cutoff := float64(carbon.Now().Timestamp()) - float64(r.retryAfter)

	script := `
		local reserved = KEYS[1]
		local queue = KEYS[2]
		local cutoff = tonumber(ARGV[1])
		local job = redis.call('ZRANGEBYSCORE', reserved, '-inf', cutoff, 'LIMIT', 0, 1)
		if #job == 0 then
			return 0
		end
		redis.call('ZREM', reserved, job[1])
		redis.call('RPUSH', queue, job[1])
		return 1
	`

	// Cap the number of jobs recovered per call so a single Pop does not
	// block on a huge backlog after a large worker crash; the next Pop
	// call continues the recovery.
	for i := 0; i < maxMigrateJobsPerPop; i++ {
		result, err := r.client.Eval(r.ctx, script, []string{reservedKey, queueKey}, cutoff).Result()
		if err != nil {
			return err
		}
		if cast.ToInt64(result) == 0 {
			break
		}
	}

	return nil
}

type QueueKey struct {
	appName    string
	connection string
}

func NewQueueKey(appName string, connection string) *QueueKey {
	return &QueueKey{
		appName:    appName,
		connection: connection,
	}
}

func (r *QueueKey) Delayed(queue string) string {
	return fmt.Sprintf("%s:delayed", r.Queue(queue))
}

func (r *QueueKey) Queue(queue string) string {
	return fmt.Sprintf("%s_queues:%s_%s", r.appName, r.connection, queue)
}

func (r *QueueKey) Reserved(queue string) string {
	return fmt.Sprintf("%s:reserved", r.Queue(queue))
}
