package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/goravel/framework/contracts/config"
	"github.com/goravel/framework/contracts/foundation"
	"github.com/goravel/framework/contracts/queue"
	"github.com/goravel/framework/errors"
	"github.com/redis/go-redis/v9"

	supportredis "github.com/goravel/redis/support/redis"
)

type Task struct {
	Uuid string   `json:"uuid"`
	Data TaskData `json:"data"`
}

type TaskData struct {
	Signature string      `json:"signature"`
	Args      []queue.Arg `json:"args"`
	Delay     *time.Time  `json:"delay"`
	Chained   []TaskData  `json:"chained"`
}

var _ queue.Driver = &Queue{}

type Queue struct {
	connection string
	ctx        context.Context
	json       foundation.Json
	queue      queue.Queue
	instance   *redis.Client
}

func NewQueue(ctx context.Context, config config.Config, queue queue.Queue, json foundation.Json, connection string) (*Queue, error) {
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
	return queue.DriverCustom
}

func (r *Queue) Later(delay time.Time, task queue.Task, queue string) error {
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

func (r *Queue) Pop(queue string) (*queue.Task, error) {
	if err := r.migrateDelayedJobs(queue); err != nil {
		return nil, err
	}

	result, err := r.instance.LPop(r.ctx, queue).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, errors.QueueDriverNoJobFound.Args(queue)
		}
		return nil, err
	}

	task, err := r.jsonToTask(result)
	if err != nil {
		return nil, err
	}

	return task, nil
}

func (r *Queue) Push(task queue.Task, queue string) error {
	if task.Data.Delay != nil && !task.Data.Delay.IsZero() {
		return r.Later(*task.Data.Delay, task, queue)
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

func (r *Queue) taskToJson(task queue.Task) ([]byte, error) {
	chained := make([]TaskData, len(task.Data.Chained))
	for i, taskData := range task.Data.Chained {
		chained[i] = TaskData{
			Signature: taskData.Job.Signature(),
			Args:      taskData.Args,
			Delay:     taskData.Delay,
		}
	}

	taskData := TaskData{
		Signature: task.Data.Job.Signature(),
		Args:      task.Data.Args,
		Delay:     task.Data.Delay,
		Chained:   chained,
	}

	payload, err := r.json.Marshal(&Task{
		Uuid: task.Uuid,
		Data: taskData,
	})
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func (r *Queue) jsonToTask(payload string) (*queue.Task, error) {
	var task Task
	if err := r.json.Unmarshal([]byte(payload), &task); err != nil {
		return nil, err
	}

	chained := make([]queue.TaskData, len(task.Data.Chained))
	for i, taskData := range task.Data.Chained {
		job, err := r.queue.GetJob(taskData.Signature)
		if err != nil {
			return nil, err
		}

		chained[i] = queue.TaskData{
			Job:   job,
			Args:  taskData.Args,
			Delay: taskData.Delay,
		}
	}

	job, err := r.queue.GetJob(task.Data.Signature)
	if err != nil {
		return nil, err
	}

	return &queue.Task{
		Uuid: task.Uuid,
		Data: queue.TaskData{
			Job:     job,
			Args:    task.Data.Args,
			Delay:   task.Data.Delay,
			Chained: chained,
		},
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
