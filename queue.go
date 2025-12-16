package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/goravel/framework/contracts/config"
	"github.com/goravel/framework/contracts/foundation"
	contractsqueue "github.com/goravel/framework/contracts/queue"
	"github.com/goravel/framework/errors"
	"github.com/redis/go-redis/v9"
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

type Queue struct {
	ctx       context.Context
	client    redis.UniversalClient
	jobStorer contractsqueue.JobStorer
	json      foundation.Json
	queueKey  *QueueKey
}

func NewQueue(ctx context.Context, config config.Config, queue contractsqueue.Queue, json foundation.Json, connection string) (*Queue, error) {
	clientConnection := config.GetString(fmt.Sprintf("queue.connections.%s.connection", connection), "default")
	client, err := GetClient(config, clientConnection)
	if err != nil {
		return nil, fmt.Errorf("failed to init redis client: %w", err)
	}

	return &Queue{
		ctx:       ctx,
		client:    client,
		jobStorer: queue.JobStorer(),
		json:      json,
		queueKey:  NewQueueKey(config.GetString("app.name", "goravel"), connection),
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

	return NewReservedJob(r.ctx, r.client, jobRecord, r.jobStorer, r.json, r.queueKey.Reserved(queue))
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
	jobs, err := r.client.ZRangeByScoreWithScores(r.ctx, delayQueueKey, &redis.ZRangeBy{
		Min:    "-inf",
		Max:    strconv.FormatFloat(float64(time.Now().Unix()), 'f', -1, 64),
		Offset: 0,
		Count:  -1,
	}).Result()
	if err != nil {
		return err
	}

	pipe := r.client.TxPipeline()
	for _, job := range jobs {
		pipe.RPush(r.ctx, queueKey, job.Member)
		pipe.ZRem(r.ctx, delayQueueKey, job.Member)
	}
	_, err = pipe.Exec(r.ctx)
	if err != nil {
		return err
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
