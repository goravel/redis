package redis

import (
	"context"
	"time"

	contractsfoundation "github.com/goravel/framework/contracts/foundation"
	contractsqueue "github.com/goravel/framework/contracts/queue"
	"github.com/goravel/framework/queue/utils"
	"github.com/goravel/framework/support/carbon"
	"github.com/redis/go-redis/v9"
)

var _ contractsqueue.ReservedJob = &ReservedJob{}

type ReservedJob struct {
	ctx              context.Context
	client           redis.UniversalClient
	jobRecord        JobRecord
	jobRecordJson    string
	jobStorer        contractsqueue.JobStorer
	json             contractsfoundation.Json
	task             contractsqueue.Task
	reservedQueueKey string
	delayedQueueKey  string
}

func NewReservedJob(ctx context.Context, client redis.UniversalClient, jobRecord JobRecord, jobStorer contractsqueue.JobStorer, json contractsfoundation.Json, reservedQueueKey string, delayedQueueKey string) (*ReservedJob, error) {
	task, err := utils.JsonToTask(jobRecord.Playload, jobStorer, json)
	if err != nil {
		return nil, err
	}

	jobRecord.Increment()
	reservedAt := jobRecord.Touch()

	jobRecordJson, err := json.MarshalString(jobRecord)
	if err != nil {
		return nil, err
	}

	if err := client.ZAdd(ctx, reservedQueueKey, redis.Z{
		Score:  float64(reservedAt.Timestamp()),
		Member: jobRecordJson,
	}).Err(); err != nil {
		return nil, err
	}

	return &ReservedJob{
		ctx:              ctx,
		client:           client,
		jobRecord:        jobRecord,
		jobRecordJson:    jobRecordJson,
		jobStorer:        jobStorer,
		json:             json,
		task:             task,
		reservedQueueKey: reservedQueueKey,
		delayedQueueKey:  delayedQueueKey,
	}, nil
}

func (r *ReservedJob) Delete() error {
	return r.client.ZRem(r.ctx, r.reservedQueueKey, r.jobRecordJson).Err()
}

// Attempts returns the number of times the job has been attempted so far.
// The count is incremented when the job was popped, so it reflects the
// persisted reservation state — matching the framework's contract that
// retry decisions survive worker restarts.
func (r *ReservedJob) Attempts() int {
	return r.jobRecord.Attempts
}

// Release removes the job from the reserved set and makes it available
// again after the delay. The serialized attempts count is preserved so
// the next pop increments it (attempt N+1). A single Lua script keeps
// the ZREM+ZADD atomic.
//
// The released member string (r.jobRecordJson) retains the original
// reserved_at from the pop — this is harmless because NewReservedJob
// overwrites it via Touch() on re-pop. We re-use the identical member
// string to guarantee the ZREM matches.
func (r *ReservedJob) Release(delay time.Duration) error {
	// Timestamp() returns integer seconds, so the fractional part of the
	// delay keeps sub-second precision through the float64 addition
	// (e.g. 500ms gives score = <seconds>.5). migrateDelayedJobs compares
	// scores with "score > now" and handles fractional scores correctly.
	score := float64(carbon.Now().Timestamp()) + delay.Seconds()

	_, err := r.client.Eval(r.ctx, `
		local reserved = KEYS[1]
		local delayed = KEYS[2]
		local score = tonumber(ARGV[1])
		local member = ARGV[2]
		redis.call('ZREM', reserved, member)
		-- The return value is a success confirmation and is intentionally ignored.
		return redis.call('ZADD', delayed, score, member)
	`, []string{r.reservedQueueKey, r.delayedQueueKey}, score, r.jobRecordJson).Result()

	return err
}

func (r *ReservedJob) Task() contractsqueue.Task {
	return r.task
}

type JobRecord struct {
	Playload   string           `json:"playload"`
	Attempts   int              `json:"attempts"`
	ReservedAt *carbon.DateTime `json:"reserved_at"`
}

func (r *JobRecord) Increment() int {
	r.Attempts++

	return r.Attempts
}

func (r *JobRecord) Touch() *carbon.DateTime {
	r.ReservedAt = carbon.NewDateTime(carbon.Now())

	return r.ReservedAt
}

func taskToJobRecordJson(task contractsqueue.Task, json contractsfoundation.Json) (string, error) {
	payload, err := utils.TaskToJson(task, json)
	if err != nil {
		return "", err
	}

	jobRecord := JobRecord{
		Playload: payload,
	}

	return json.MarshalString(jobRecord)
}

func jsonToJobRecord(payload string, json contractsfoundation.Json) (JobRecord, error) {
	var jobRecord JobRecord
	if err := json.UnmarshalString(payload, &jobRecord); err != nil {
		return JobRecord{}, err
	}

	return jobRecord, nil
}
