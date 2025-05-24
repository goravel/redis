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
	frameworkqueue "github.com/goravel/framework/queue"
	"github.com/redis/go-redis/v9"

	supportredis "github.com/goravel/redis/support/redis"
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
	ctx      context.Context
	json     foundation.Json
	queue    contractsqueue.Queue
	instance *redis.Client

	appName    string
	connection string
}

func NewQueue(ctx context.Context, config config.Config, queue contractsqueue.Queue, json foundation.Json, connection string) (*Queue, error) {
	clientConnection := config.GetString(fmt.Sprintf("queue.connections.%s.connection", connection), "default")
	client, err := supportredis.GetClient(config, clientConnection)
	if err != nil {
		return nil, fmt.Errorf("failed to init redis client: %w", err)
	}

	return &Queue{
		ctx:      ctx,
		json:     json,
		queue:    queue,
		instance: client,

		appName:    config.GetString("app.name", "goravel"),
		connection: connection,
	}, nil
}

func (r *Queue) Driver() string {
	return contractsqueue.DriverCustom
}

func (r *Queue) Later(delay time.Time, task contractsqueue.Task, queue string) error {
	payload, err := frameworkqueue.TaskToJson(task, r.json)
	if err != nil {
		return err
	}

	return r.instance.ZAdd(r.ctx, r.delayQueueKey(queue), redis.Z{
		Score:  float64(delay.Unix()),
		Member: payload,
	}).Err()
}

func (r *Queue) Pop(queue string) (contractsqueue.Task, error) {
	queueKey := r.queueKey(queue)

	if err := r.migrateDelayedJobs(queue); err != nil {
		return contractsqueue.Task{}, err
	}

	result, err := r.instance.LPop(r.ctx, queueKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return contractsqueue.Task{}, errors.QueueDriverNoJobFound.Args(queueKey)
		}
		return contractsqueue.Task{}, err
	}

	return frameworkqueue.JsonToTask(result, r.queue, r.json)
}

func (r *Queue) Push(task contractsqueue.Task, queue string) error {
	if !task.Delay.IsZero() {
		return r.Later(task.Delay, task, queue)
	}

	payload, err := frameworkqueue.TaskToJson(task, r.json)
	if err != nil {
		return err
	}

	return r.instance.RPush(r.ctx, r.queueKey(queue), payload).Err()
}

func (r *Queue) delayQueueKey(queue string) string {
	return fmt.Sprintf("%s:delayed", r.queueKey(queue))
}

func (r *Queue) queueKey(queue string) string {
	return fmt.Sprintf("%s_queues:%s_%s", r.appName, r.connection, queue)
}

func (r *Queue) migrateDelayedJobs(queue string) error {
	queueKey := r.queueKey(queue)
	delayQueueKey := r.delayQueueKey(queue)
	jobs, err := r.instance.ZRangeByScoreWithScores(r.ctx, delayQueueKey, &redis.ZRangeBy{
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
		pipe.RPush(r.ctx, queueKey, job.Member)
		pipe.ZRem(r.ctx, delayQueueKey, job.Member)
	}
	_, err = pipe.Exec(r.ctx)
	if err != nil {
		return err
	}

	return nil
}
