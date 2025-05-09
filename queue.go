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
	"github.com/goravel/framework/queue"
	"github.com/redis/go-redis/v9"

	supportredis "github.com/goravel/redis/support/redis"
)

type Task struct {
	Job
	Uuid  string `json:"uuid"`
	Chain []Job  `json:"chain"`
}

type Job struct {
	Signature string               `json:"signature"`
	Args      []contractsqueue.Arg `json:"args"`
	Delay     *time.Time           `json:"delay"`
}

var _ contractsqueue.Driver = &Queue{}

type Queue struct {
	connection string
	ctx        context.Context
	json       foundation.Json
	queue      contractsqueue.Queue
	instance   *redis.Client
}

func NewQueue(ctx context.Context, config config.Config, queue contractsqueue.Queue, json foundation.Json, connection string) (*Queue, error) {
	connection = config.GetString(fmt.Sprintf("queue.connections.%s.connection", connection), "default")

	client, err := supportredis.GetClient(config, connection)
	if err != nil {
		return nil, fmt.Errorf("failed to init redis client: %w", err)
	}

	return &Queue{
		connection: connection,
		ctx:        ctx,
		json:       json,
		queue:      queue,
		instance:   client,
	}, nil
}

func (r *Queue) Connection() string {
	return r.connection
}

func (r *Queue) Driver() string {
	return contractsqueue.DriverCustom
}

func (r *Queue) Instance() *redis.Client {
	return r.instance
}

func (r *Queue) Later(delay time.Time, task contractsqueue.Task, queueKey string) error {
	payload, err := queue.TaskToJson(task, r.json)
	if err != nil {
		return err
	}

	return r.instance.ZAdd(r.ctx, r.delayQueueKey(queueKey), redis.Z{
		Score:  float64(delay.Unix()),
		Member: payload,
	}).Err()
}

func (r *Queue) Name() string {
	return Name
}

func (r *Queue) Pop(queueKey string) (contractsqueue.Task, error) {
	if err := r.migrateDelayedJobs(queueKey); err != nil {
		return contractsqueue.Task{}, err
	}

	result, err := r.instance.LPop(r.ctx, queueKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return contractsqueue.Task{}, errors.QueueDriverNoJobFound.Args(queueKey)
		}
		return contractsqueue.Task{}, err
	}

	return queue.JsonToTask(result, r.queue, r.json)
}

func (r *Queue) Push(task contractsqueue.Task, queueKey string) error {
	if !task.Delay.IsZero() {
		return r.Later(task.Delay, task, queueKey)
	}

	payload, err := queue.TaskToJson(task, r.json)
	if err != nil {
		return err
	}

	return r.instance.RPush(r.ctx, queueKey, payload).Err()
}

func (r *Queue) delayQueueKey(queue string) string {
	return fmt.Sprintf("%s:delayed", queue)
}

func (r *Queue) migrateDelayedJobs(queue string) error {
	jobs, err := r.instance.ZRangeByScoreWithScores(r.ctx, r.delayQueueKey(queue), &redis.ZRangeBy{
		Min:    "-inf",
		Max:    strconv.FormatFloat(float64(time.Now().Unix()), 'f', -1, 64),
		Offset: 0,
		Count:  -1,
	}).Result()
	if err != nil {
		return err
	}

	pipe := r.instance.TxPipeline()
	for _, job := range jobs {
		pipe.RPush(r.ctx, queue, job.Member)
		pipe.ZRem(r.ctx, r.delayQueueKey(queue), job.Member)
	}
	_, err = pipe.Exec(r.ctx)
	if err != nil {
		return err
	}

	return nil
}
