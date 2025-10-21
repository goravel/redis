package redis

import (
	"context"

	contractsfoundation "github.com/goravel/framework/contracts/foundation"
	contractsqueue "github.com/goravel/framework/contracts/queue"
	"github.com/goravel/framework/queue/utils"
	"github.com/goravel/framework/support/carbon"
	"github.com/redis/go-redis/v9"
)

type ReservedJob struct {
	ctx              context.Context
	client           redis.UniversalClient
	jobRecord        JobRecord
	jobRecordJson    string
	jobStorer        contractsqueue.JobStorer
	json             contractsfoundation.Json
	task             contractsqueue.Task
	reservedQueueKey string
}

func NewReservedJob(ctx context.Context, client redis.UniversalClient, jobRecord JobRecord, jobStorer contractsqueue.JobStorer, json contractsfoundation.Json, reservedQueueKey string) (*ReservedJob, error) {
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
	}, nil
}

func (r *ReservedJob) Delete() error {
	return r.client.ZRem(r.ctx, r.reservedQueueKey, r.jobRecordJson).Err()
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
