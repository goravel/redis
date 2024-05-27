package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"

	"github.com/goravel/framework/contracts/config"
	contractsqueue "github.com/goravel/framework/contracts/queue"
	"github.com/goravel/framework/queue"
	"github.com/goravel/framework/support/json"
)

type queueData struct {
	Signature string `json:"signature"`
	Args      []any  `json:"args"`
	Attempts  uint   `json:"attempts"`
}

var _ contractsqueue.Driver = &Queue{}

type Queue struct {
	ctx        context.Context
	queue      contractsqueue.Queue
	instance   *redis.Client
	connection string
}

func NewQueue(ctx context.Context, config config.Config, queue contractsqueue.Queue, connection string) (*Queue, error) {
	redisConnection := config.GetString(fmt.Sprintf("queue.connections.%s.connection", connection), "default")
	host := config.GetString(fmt.Sprintf("database.redis.%s.host", redisConnection))
	if host == "" {
		return nil, nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, config.GetString(fmt.Sprintf("database.redis.%s.port", redisConnection))),
		Password: config.GetString(fmt.Sprintf("database.redis.%s.password", redisConnection)),
		DB:       config.GetInt(fmt.Sprintf("database.redis.%s.database", redisConnection)),
	})

	if _, err := client.Ping(context.Background()).Result(); err != nil {
		return nil, errors.WithMessage(err, "init connection error")
	}

	return &Queue{
		ctx:        ctx,
		queue:      queue,
		instance:   client,
		connection: connection,
	}, nil
}

func (r *Queue) Connection() string {
	return r.connection
}

func (r *Queue) Driver() string {
	return queue.DriverCustom
}

func (r *Queue) Push(job contractsqueue.Job, args []any, queue string) error {
	payload, err := r.jobToJSON(job.Signature(), args)
	if err != nil {
		return err
	}

	return r.instance.RPush(r.ctx, queue, payload).Err()
}

func (r *Queue) Bulk(jobs []contractsqueue.Jobs, queue string) error {
	pipe := r.instance.Pipeline()

	for _, job := range jobs {
		payload, err := r.jobToJSON(job.Job.Signature(), job.Args)
		if err != nil {
			return err
		}

		if job.Delay > 0 {
			delayDuration := time.Duration(job.Delay) * time.Second
			pipe.ZAdd(r.ctx, queue+":delayed", redis.Z{
				Score:  float64(time.Now().Add(delayDuration).Unix()),
				Member: payload,
			})
		} else {
			pipe.RPush(r.ctx, queue, payload)
		}
	}

	_, err := pipe.Exec(r.ctx)
	return err
}

func (r *Queue) Later(delay uint, job contractsqueue.Job, args []any, queue string) error {
	payload, err := r.jobToJSON(job.Signature(), args)
	if err != nil {
		return err
	}

	delayDuration := time.Duration(delay) * time.Second
	return r.instance.ZAdd(r.ctx, queue+":delayed", redis.Z{
		Score:  float64(time.Now().Add(delayDuration).Unix()),
		Member: payload,
	}).Err()
}

func (r *Queue) Pop(queue string) (contractsqueue.Job, []any, error) {
	if err := r.migrateDelayedJobs(queue); err != nil {
		return nil, nil, err
	}

	result, err := r.instance.LPop(r.ctx, queue).Result()
	if err != nil {
		return nil, nil, err
	}

	signature, args, err := r.jsonToJob(result)
	if err != nil {
		return nil, nil, err
	}
	job, err := r.queue.GetJob(signature)
	if err != nil {
		return nil, nil, err
	}

	return job, args, nil
}

func (r *Queue) migrateDelayedJobs(queue string) error {
	jobs, err := r.instance.ZRangeByScoreWithScores(r.ctx, queue+":delayed", &redis.ZRangeBy{
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
		pipe.ZRem(r.ctx, queue+":delayed", job.Member)
	}
	_, err = pipe.Exec(r.ctx)
	if err != nil {
		return err
	}

	return nil
}

// jobToJSON convert signature and args to JSON
func (r *Queue) jobToJSON(signature string, args []any) (string, error) {
	return json.MarshalString(&queueData{
		Signature: signature,
		Args:      args,
		Attempts:  0,
	})
}

// jsonToJob convert JSON to signature and args
func (r *Queue) jsonToJob(jsonString string) (string, []any, error) {
	var data queueData
	err := json.UnmarshalString(jsonString, &data)
	if err != nil {
		return "", nil, err
	}

	return data.Signature, data.Args, nil
}
