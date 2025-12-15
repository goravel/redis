package redis

import (
	"context"
	"fmt"
	"testing"

	contractsqueue "github.com/goravel/framework/contracts/queue"
	"github.com/goravel/framework/foundation/json"
	mocksconfig "github.com/goravel/framework/mocks/config"
	mocksqueue "github.com/goravel/framework/mocks/queue"
	"github.com/goravel/framework/support/carbon"
	"github.com/goravel/framework/support/env"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type ReservedJobTestSuite struct {
	suite.Suite
	ctx              context.Context
	client           redis.UniversalClient
	mockJobStorer    *mocksqueue.JobStorer
	docker           *Docker
	reservedQueueKey string
}

func TestReservedJobTestSuite(t *testing.T) {
	if env.IsWindows() {
		t.Skip("Skipping tests of using docker")
	}

	suite.Run(t, &ReservedJobTestSuite{})
}

func (s *ReservedJobTestSuite) SetupSuite() {
	carbon.SetTestNow(carbon.Now())

	mockConfig := mocksconfig.NewConfig(s.T())
	docker := initDocker(mockConfig)

	client, err := docker.connect()
	s.Require().NoError(err)

	s.ctx = context.Background()
	s.client = client
	s.docker = docker
	s.mockJobStorer = mocksqueue.NewJobStorer(s.T())
	s.reservedQueueKey = "test-reserved-queue"
}

func (s *ReservedJobTestSuite) TearDownSuite() {
	s.NoError(s.docker.Shutdown())
	carbon.ClearTestNow()
}

func (s *ReservedJobTestSuite) SetupTest() {}

func (s *ReservedJobTestSuite) TestNewReservedJob() {
	jobRecord := JobRecord{
		Playload: "{\"uuid\":\"865111de-ff50-4652-9733-72fea655f836\",\"signature\":\"mock\",\"args\":[{\"type\":\"[]string\",\"value\":[\"test\",\"test2\",\"test3\"]}],\"delay\":\"2025-05-28T19:50:57Z\"}",
	}

	s.mockJobStorer.EXPECT().Get("mock").Return(&MockJob{}, nil).Once()

	reservedJob, err := NewReservedJob(s.ctx, s.client, jobRecord, s.mockJobStorer, json.New(), s.reservedQueueKey)
	s.NoError(err)
	s.NotNil(reservedJob)
	s.Equal(s.ctx, reservedJob.ctx)
	s.Equal(s.client, reservedJob.client)
	s.Equal(s.reservedQueueKey, reservedJob.reservedQueueKey)
	s.Equal(jobRecord.Playload, reservedJob.jobRecord.Playload)
	s.Equal(fmt.Sprintf("{\"playload\":\"{\\\"uuid\\\":\\\"865111de-ff50-4652-9733-72fea655f836\\\",\\\"signature\\\":\\\"mock\\\",\\\"args\\\":[{\\\"type\\\":\\\"[]string\\\",\\\"value\\\":[\\\"test\\\",\\\"test2\\\",\\\"test3\\\"]}],\\\"delay\\\":\\\"2025-05-28T19:50:57Z\\\"}\",\"attempts\":1,\"reserved_at\":\"%s\"}", carbon.Now().ToDateTimeString()), reservedJob.jobRecordJson)
	s.Equal(1, reservedJob.jobRecord.Attempts)                                  // Should be incremented
	s.Equal(carbon.NewDateTime(carbon.Now()), reservedJob.jobRecord.ReservedAt) // Should be set
	s.Equal(s.mockJobStorer, reservedJob.jobStorer)
	s.NotNil(reservedJob.json)
	s.Equal(contractsqueue.Task{
		UUID: "865111de-ff50-4652-9733-72fea655f836",
		ChainJob: contractsqueue.ChainJob{
			Job: &MockJob{},
			Args: []contractsqueue.Arg{
				{
					Type:  "[]string",
					Value: []any{"test", "test2", "test3"},
				},
			},
			Delay: carbon.Parse("2025-05-28T19:50:57Z").StdTime(),
		},
	}, reservedJob.task)
}

func (s *ReservedJobTestSuite) Test_Delete() {
	jobRecord := JobRecord{
		Playload: "{\"uuid\":\"865111de-ff50-4652-9733-72fea655f836\",\"signature\":\"mock\",\"args\":[{\"type\":\"[]string\",\"value\":[\"test\",\"test2\",\"test3\"]}],\"delay\":\"2025-05-28T19:50:57Z\"}",
	}

	s.mockJobStorer.EXPECT().Get("mock").Return(&MockJob{}, nil).Once()

	reservedJob, err := NewReservedJob(s.ctx, s.client, jobRecord, s.mockJobStorer, json.New(), s.reservedQueueKey)
	s.NoError(err)

	count, err := s.client.ZCount(context.Background(), s.reservedQueueKey, "-inf", "+inf").Result()
	s.NoError(err)
	s.Equal(int64(1), count)

	err = reservedJob.Delete()
	s.NoError(err)

	count, err = s.client.ZCount(context.Background(), s.reservedQueueKey, "-inf", "+inf").Result()
	s.NoError(err)
	s.Equal(int64(0), count)
}

func TestJobRecord(t *testing.T) {
	carbon.SetTestNow(carbon.Now())
	defer carbon.ClearTestNow()

	jobRecord := JobRecord{
		Playload: "{}",
	}

	jobRecord.Increment()
	assert.Equal(t, 1, jobRecord.Attempts)

	jobRecord.Touch()
	assert.Equal(t, carbon.NewDateTime(carbon.Now()), jobRecord.ReservedAt)

	jobRecord.Increment()
	assert.Equal(t, 2, jobRecord.Attempts)
}

func TestTaskToJobRecordJson(t *testing.T) {
	task := contractsqueue.Task{
		UUID: "test-uuid",
		ChainJob: contractsqueue.ChainJob{
			Job: &MockJob{},
		},
	}

	json, err := taskToJobRecordJson(task, json.New())

	assert.NoError(t, err)
	assert.Equal(t, `{"playload":"{\"delay\":null,\"signature\":\"mock\",\"args\":null,\"uuid\":\"test-uuid\",\"chain\":null}","attempts":0,"reserved_at":null}`, json)
}
