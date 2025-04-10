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

type task struct {
	Signature string      `json:"signature"`
	Args      []queue.Arg `json:"args"`
	Attempts  uint        `json:"attempts"`
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

func (r *Queue) Bulk(jobs []queue.Jobs, queue string) error {
	pipe := r.instance.Pipeline()
	now := time.Now()

	for _, job := range jobs {
		payload, err := r.jobToJson(job.Job, job.Args)
		if err != nil {
			return err
		}

		if job.Delay.After(now) {
			pipe.ZAdd(r.ctx, r.delayQueueKey(queue), redis.Z{
				Score:  float64(job.Delay.Unix()),
				Member: payload,
			})
		} else {
			pipe.RPush(r.ctx, queue, payload)
		}
	}

	_, err := pipe.Exec(r.ctx)
	return err
}

func (r *Queue) Connection() string {
	return r.connection
}

func (r *Queue) Driver() string {
	return queue.DriverCustom
}

func (r *Queue) Later(delay time.Time, job queue.Job, args []queue.Arg, queue string) error {
	payload, err := r.jobToJson(job, args)
	if err != nil {
		return err
	}

	return r.instance.ZAdd(r.ctx, r.delayQueueKey(queue), redis.Z{
		Score:  float64(delay.Unix()),
		Member: payload,
	}).Err()
}

func (r *Queue) Pop(queue string) (queue.Job, []queue.Arg, error) {
	if err := r.migrateDelayedJobs(queue); err != nil {
		return nil, nil, err
	}

	result, err := r.instance.LPop(r.ctx, queue).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil, errors.QueueDriverNoJobFound.Args(queue)
		}
		return nil, nil, err
	}

	job, args, err := r.jsonToJob(result)
	if err != nil {
		return nil, nil, err
	}

	return job, args, nil
}

func (r *Queue) Push(job queue.Job, args []queue.Arg, queue string) error {
	payload, err := r.jobToJson(job, args)
	if err != nil {
		return err
	}

	return r.instance.RPush(r.ctx, queue, payload).Err()
}

func (r *Queue) delayQueueKey(queue string) string {
	return fmt.Sprintf("%s:delayed", queue)
}

// jobToJson converts job and args to json
func (r *Queue) jobToJson(job queue.Job, args []queue.Arg) ([]byte, error) {
	payload, err := r.json.Marshal(&task{
		Signature: job.Signature(),
		Args:      args,
		Attempts:  0,
	})
	if err != nil {
		return nil, err
	}

	return payload, nil
}

// jsonToJob converts json to job and args
func (r *Queue) jsonToJob(payload string) (queue.Job, []queue.Arg, error) {
	var dst task
	if err := r.json.Unmarshal([]byte(payload), &dst); err != nil {
		return nil, nil, err
	}

	job, err := r.queue.GetJob(dst.Signature)
	if err != nil {
		return nil, nil, err
	}

	return job, dst.Args, nil
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
