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
	"github.com/spf13/cast"

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

func (r *Queue) Later(delay time.Time, task contractsqueue.Task, queue string) error {
	payload, err := r.taskToJson(task)
	if err != nil {
		return err
	}

	return r.instance.ZAdd(r.ctx, r.delayQueueKey(queue), redis.Z{
		Score:  float64(delay.Unix()),
		Member: payload,
	}).Err()
}

func (r *Queue) Name() string {
	return Name
}

func (r *Queue) Pop(queue string) (contractsqueue.Task, error) {
	if err := r.migrateDelayedJobs(queue); err != nil {
		return contractsqueue.Task{}, err
	}

	result, err := r.instance.LPop(r.ctx, queue).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return contractsqueue.Task{}, errors.QueueDriverNoJobFound.Args(queue)
		}
		return contractsqueue.Task{}, err
	}

	return r.jsonToTask(result)
}

func (r *Queue) Push(task contractsqueue.Task, queue string) error {
	if !task.Delay.IsZero() {
		return r.Later(task.Delay, task, queue)
	}

	payload, err := r.taskToJson(task)
	if err != nil {
		return err
	}

	return r.instance.RPush(r.ctx, queue, payload).Err()
}

func (r *Queue) delayQueueKey(queue string) string {
	return fmt.Sprintf("%s:delayed", queue)
}

func (r *Queue) taskToJson(task contractsqueue.Task) ([]byte, error) {
	chain := make([]Job, len(task.Chain))
	for i, taskData := range task.Chain {
		for j, arg := range taskData.Args {
			// To avoid converting []uint8 to base64
			if arg.Type == "[]uint8" {
				taskData.Args[j].Value = cast.ToIntSlice(arg.Value)
			}
		}

		job := Job{
			Signature: taskData.Job.Signature(),
			Args:      taskData.Args,
		}

		if !taskData.Delay.IsZero() {
			job.Delay = &taskData.Delay
		}

		chain[i] = job
	}

	var args []contractsqueue.Arg
	for _, arg := range task.Args {
		if arg.Type == "[]uint8" {
			arg.Value = cast.ToIntSlice(arg.Value)
		}
		args = append(args, arg)
	}

	job := Job{
		Signature: task.Job.Signature(),
		Args:      args,
	}

	if !task.Delay.IsZero() {
		job.Delay = &task.Delay
	}

	t := Task{
		Uuid:  task.UUID,
		Job:   job,
		Chain: chain,
	}

	payload, err := r.json.Marshal(t)
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func (r *Queue) jsonToTask(payload string) (contractsqueue.Task, error) {
	var task Task
	if err := r.json.Unmarshal([]byte(payload), &task); err != nil {
		return contractsqueue.Task{}, err
	}

	chain := make([]contractsqueue.Jobs, len(task.Chain))
	for i, item := range task.Chain {
		job, err := r.queue.GetJob(item.Signature)
		if err != nil {
			return contractsqueue.Task{}, err
		}

		jobs := contractsqueue.Jobs{
			Job:  job,
			Args: item.Args,
		}

		if item.Delay != nil && !item.Delay.IsZero() {
			jobs.Delay = *item.Delay
		}

		chain[i] = jobs
	}

	job, err := r.queue.GetJob(task.Signature)
	if err != nil {
		return contractsqueue.Task{}, err
	}

	jobs := contractsqueue.Jobs{
		Job:  job,
		Args: task.Args,
	}

	if task.Delay != nil && !task.Delay.IsZero() {
		jobs.Delay = *task.Delay
	}

	return contractsqueue.Task{
		UUID:  task.Uuid,
		Jobs:  jobs,
		Chain: chain,
	}, nil
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
