package redis

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/goravel/framework/contracts/config"
	"github.com/goravel/framework/contracts/queue"
	frameworkerrors "github.com/goravel/framework/errors"
	"github.com/redis/go-redis/v9"
)

func init() {
	gob.Register(queueData{})
}

type queueData struct {
	Signature string `json:"signature"`
	Args      []any  `json:"args"`
	Attempts  uint   `json:"attempts"`
}

var _ queue.Driver = &Queue{}

type Queue struct {
	ctx        context.Context
	queue      queue.Queue
	instance   *redis.Client
	connection string
}

func NewQueue(ctx context.Context, config config.Config, queue queue.Queue, connection string) (*Queue, error) {
	connection = config.GetString(fmt.Sprintf("queue.connections.%s.connection", connection), "default")
	host := config.GetString(fmt.Sprintf("database.redis.%s.host", connection))
	if host == "" {
		return nil, fmt.Errorf("redis host is not configured for connection %s", connection)
	}

	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, config.GetString(fmt.Sprintf("database.redis.%s.port", connection))),
		Username: config.GetString(fmt.Sprintf("database.redis.%s.username", connection)),
		Password: config.GetString(fmt.Sprintf("database.redis.%s.password", connection)),
		DB:       config.GetInt(fmt.Sprintf("database.redis.%s.database", connection)),
	})

	if _, err := client.Ping(context.Background()).Result(); err != nil {
		return nil, err
	}

	return &Queue{
		ctx:        ctx,
		queue:      queue,
		instance:   client,
		connection: connection,
	}, nil
}

func (r *Queue) Bulk(jobs []queue.Jobs, queue string) error {
	pipe := r.instance.Pipeline()
	now := time.Now()

	for _, job := range jobs {
		payload, err := r.jobToGob(job.Job.Signature(), job.Args)
		if err != nil {
			return err
		}

		if job.Delay.After(now) {
			pipe.ZAdd(r.ctx, queue+":delayed", redis.Z{
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

func (r *Queue) Pop(queue string) (queue.Job, []any, error) {
	if err := r.migrateDelayedJobs(queue); err != nil {
		return nil, nil, err
	}

	result, err := r.instance.LPop(r.ctx, queue).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil, frameworkerrors.QueueDriverNoJobFound.Args(queue)
		}
		return nil, nil, err
	}

	signature, args, err := r.gobToJob(result)
	if err != nil {
		return nil, nil, err
	}
	job, err := r.queue.GetJob(signature)
	if err != nil {
		return nil, nil, err
	}

	return job, args, nil
}

func (r *Queue) Push(job queue.Job, args []any, queue string) error {
	payload, err := r.jobToGob(job.Signature(), args)
	if err != nil {
		return err
	}

	return r.instance.RPush(r.ctx, queue, payload).Err()
}

func (r *Queue) Later(delay time.Time, job queue.Job, args []any, queue string) error {
	payload, err := r.jobToGob(job.Signature(), args)
	if err != nil {
		return err
	}

	return r.instance.ZAdd(r.ctx, queue+":delayed", redis.Z{
		Score:  float64(delay.Unix()),
		Member: payload,
	}).Err()
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

// jobToGob convert signature and args to Gob
func (r *Queue) jobToGob(signature string, args []any) ([]byte, error) {
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	if err := enc.Encode(&queueData{
		Signature: signature,
		Args:      args,
		Attempts:  0,
	}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// gobToJob convert Gob to signature and args
func (r *Queue) gobToJob(src []byte) (string, []any, error) {
	dst := new(queueData)
	dec := gob.NewDecoder(bytes.NewBuffer(src))
	if err := dec.Decode(dst); err != nil {
		return "", nil, err
	}

	return dst.Signature, dst.Args, nil
}
