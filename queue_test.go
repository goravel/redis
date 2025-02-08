package redis

import (
	"context"
	"testing"
	"time"

	contractsqueue "github.com/goravel/framework/contracts/queue"
	mocksconfig "github.com/goravel/framework/mocks/config"
	mocksorm "github.com/goravel/framework/mocks/database/orm"
	"github.com/goravel/framework/queue"
	"github.com/goravel/framework/support/docker"
	"github.com/goravel/framework/support/env"
	"github.com/spf13/cast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var (
	testRedisJob       = 0
	testDelayRedisJob  = 0
	testCustomRedisJob = 0
	testErrorRedisJob  = 0
	testChainRedisJob  = 0
)

type QueueTestSuite struct {
	suite.Suite
	app         *queue.Application
	mockConfig  *mocksconfig.Config
	redis       *Queue
	redisDocker *docker.Redis
}

func TestQueueTestSuite(t *testing.T) {
	if env.IsWindows() {
		t.Skip("Skipping tests of using docker")
	}

	redisDocker := docker.NewRedis()
	assert.Nil(t, redisDocker.Build())

	mockConfig := mocksconfig.NewConfig(t)
	mockConfig.EXPECT().GetString("queue.connections.redis.connection", "default").Return("default").Once()
	mockConfig.EXPECT().GetString("database.redis.default.host").Return("localhost").Once()
	mockConfig.EXPECT().GetString("database.redis.default.port").Return(cast.ToString(redisDocker.Config().Port)).Once()
	mockConfig.EXPECT().GetString("database.redis.default.username").Return("").Once()
	mockConfig.EXPECT().GetString("database.redis.default.password").Return("").Once()
	mockConfig.EXPECT().GetInt("database.redis.default.database").Return(0).Once()
	app := queue.NewApplication(mockConfig)
	app.Register([]contractsqueue.Job{&TestRedisJob{}, &TestDelayRedisJob{}, &TestCustomRedisJob{}, &TestErrorRedisJob{}, &TestChainRedisJob{}})

	redis, err := NewQueue(context.Background(), mockConfig, app, "redis")
	assert.Nil(t, err)

	suite.Run(t, &QueueTestSuite{
		app:         app,
		mockConfig:  mockConfig,
		redisDocker: redisDocker,
		redis:       redis,
	})
}

func (s *QueueTestSuite) SetupTest() {
	testRedisJob = 0
	testDelayRedisJob = 0
	testCustomRedisJob = 0
	testErrorRedisJob = 0
	testChainRedisJob = 0

	mockOrm := mocksorm.NewOrm(s.T())
	mockQuery := mocksorm.NewQuery(s.T())
	mockOrm.EXPECT().Connection("database").Return(mockOrm)
	mockOrm.EXPECT().Query().Return(mockQuery)
	mockQuery.EXPECT().Table("failed_jobs").Return(mockQuery)

	queue.OrmFacade = mockOrm
}

func (s *QueueTestSuite) TestDefaultRedisQueue() {
	s.mockConfig.EXPECT().GetString("queue.default").Return("redis").Once()
	s.mockConfig.EXPECT().GetString("app.name").Return("goravel").Once()
	s.mockConfig.EXPECT().GetString("queue.connections.redis.queue", "default").Return("default").Once()
	s.mockConfig.EXPECT().GetString("queue.connections.redis.driver").Return("custom").Once()
	s.mockConfig.EXPECT().Get("queue.connections.redis.via").Return(func() (contractsqueue.Driver, error) { return s.redis, nil }).Once()
	s.mockConfig.EXPECT().GetString("database.redis.default.host").Return("localhost").Once()
	s.mockConfig.EXPECT().GetString("database.redis.default.username").Return("").Once()
	s.mockConfig.EXPECT().GetString("database.redis.default.password").Return("").Once()
	s.mockConfig.EXPECT().GetInt("database.redis.default.database").Return(0).Once()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	worker := s.app.Worker()
	defer s.Nil(worker.Shutdown())
	go func(ctx context.Context) {
		s.Nil(worker.Run())
	}(ctx)

	time.Sleep(2 * time.Second)
	s.Nil(s.app.Job(&TestRedisJob{}, []any{"TestDefaultRedisQueue", 1}).Dispatch())
	time.Sleep(2 * time.Second)
	s.Equal(1, testRedisJob)
}

func (s *QueueTestSuite) TestDelayRedisQueue() {
	s.mockConfig.EXPECT().GetString("queue.default").Return("redis").Once()
	s.mockConfig.EXPECT().GetString("app.name").Return("goravel").Once()
	s.mockConfig.EXPECT().GetString("queue.connections.redis.queue", "default").Return("default").Once()
	s.mockConfig.EXPECT().GetString("queue.connections.redis.driver").Return("custom").Once()
	s.mockConfig.EXPECT().Get("queue.connections.redis.via").Return(func() (contractsqueue.Driver, error) { return s.redis, nil }).Once()
	s.mockConfig.EXPECT().GetString("database.redis.default.host").Return("localhost").Once()
	s.mockConfig.EXPECT().GetString("database.redis.default.username").Return("").Once()
	s.mockConfig.EXPECT().GetString("database.redis.default.password").Return("").Once()
	s.mockConfig.EXPECT().GetInt("database.redis.default.database").Return(0).Once()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	worker := s.app.Worker(contractsqueue.Args{
		Queue: "delay",
	})
	defer s.Nil(worker.Shutdown())
	go func(ctx context.Context) {
		s.Nil(worker.Run())
	}(ctx)

	time.Sleep(2 * time.Second)
	s.Nil(s.app.Job(&TestDelayRedisJob{}, []any{"TestDelayRedisQueue", 1}).OnQueue("delay").Delay(time.Now().Add(3 * time.Second)).Dispatch())
	time.Sleep(2 * time.Second)
	s.Equal(0, testDelayRedisJob)
	time.Sleep(3 * time.Second)
	s.Equal(1, testDelayRedisJob)
}

func (s *QueueTestSuite) TestCustomRedisQueue() {
	s.mockConfig.EXPECT().GetString("queue.default").Return("custom").Once()
	s.mockConfig.EXPECT().GetString("app.name").Return("goravel").Once()
	s.mockConfig.EXPECT().GetString("queue.connections.custom.driver").Return("custom").Once()
	s.mockConfig.EXPECT().Get("queue.connections.custom.via").Return(func() (contractsqueue.Driver, error) { return s.redis, nil }).Once()
	s.mockConfig.EXPECT().GetString("database.redis.default.host").Return("localhost").Once()
	s.mockConfig.EXPECT().GetString("database.redis.default.username").Return("").Once()
	s.mockConfig.EXPECT().GetString("database.redis.default.password").Return("").Once()
	s.mockConfig.EXPECT().GetInt("database.redis.default.database").Return(0).Once()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	worker := s.app.Worker(contractsqueue.Args{
		Connection: "custom",
		Queue:      "custom1",
		Concurrent: 2,
	})
	defer s.Nil(worker.Shutdown())
	go func(ctx context.Context) {
		s.Nil(worker.Run())
	}(ctx)

	time.Sleep(2 * time.Second)
	s.Nil(s.app.Job(&TestCustomRedisJob{}, []any{"TestCustomRedisQueue", 1}).OnConnection("custom").OnQueue("custom1").Dispatch())
	time.Sleep(2 * time.Second)
	s.Equal(1, testCustomRedisJob)
}

func (s *QueueTestSuite) TestErrorRedisQueue() {
	s.mockConfig.EXPECT().GetString("queue.default").Return("redis").Once()
	s.mockConfig.EXPECT().GetString("app.name").Return("goravel").Once()
	s.mockConfig.EXPECT().GetString("queue.connections.redis.queue", "default").Return("default").Once()
	s.mockConfig.EXPECT().GetString("queue.connections.redis.driver").Return("custom").Once()
	s.mockConfig.EXPECT().Get("queue.connections.redis.via").Return(func() (contractsqueue.Driver, error) { return s.redis, nil }).Once()
	s.mockConfig.EXPECT().GetString("database.redis.default.host").Return("localhost").Once()
	s.mockConfig.EXPECT().GetString("database.redis.default.username").Return("").Once()
	s.mockConfig.EXPECT().GetString("database.redis.default.password").Return("").Once()
	s.mockConfig.EXPECT().GetInt("database.redis.default.database").Return(0).Once()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	worker := s.app.Worker(contractsqueue.Args{
		Queue: "error",
	})
	defer s.Nil(worker.Shutdown())
	go func(ctx context.Context) {
		s.Nil(worker.Run())
	}(ctx)

	time.Sleep(2 * time.Second)
	s.Nil(s.app.Job(&TestErrorRedisJob{}, []any{"TestErrorRedisQueue", 1}).OnConnection("redis").OnQueue("error1").Dispatch())
	time.Sleep(2 * time.Second)
	s.Equal(0, testErrorRedisJob)
}

func (s *QueueTestSuite) TestChainRedisQueue() {
	s.mockConfig.EXPECT().GetString("queue.default").Return("redis").Once()
	s.mockConfig.EXPECT().GetString("app.name").Return("goravel").Once()
	s.mockConfig.EXPECT().GetString("queue.connections.redis.queue", "default").Return("default").Once()
	s.mockConfig.EXPECT().GetString("queue.connections.redis.driver").Return("custom").Once()
	s.mockConfig.EXPECT().Get("queue.connections.redis.via").Return(func() (contractsqueue.Driver, error) { return s.redis, nil }).Once()
	s.mockConfig.EXPECT().GetString("database.redis.default.host").Return("localhost").Once()
	s.mockConfig.EXPECT().GetString("database.redis.default.username").Return("").Once()
	s.mockConfig.EXPECT().GetString("database.redis.default.password").Return("").Once()
	s.mockConfig.EXPECT().GetInt("database.redis.default.database").Return(0).Once()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	worker := s.app.Worker(contractsqueue.Args{
		Queue: "chain",
	})
	defer s.Nil(worker.Shutdown())
	go func(ctx context.Context) {
		s.Nil(worker.Run())
	}(ctx)

	time.Sleep(2 * time.Second)
	s.Nil(s.app.Chain([]contractsqueue.Jobs{
		{
			Job:  &TestChainRedisJob{},
			Args: []any{"TestChainRedisJob", 1},
		},
		{
			Job:  &TestRedisJob{},
			Args: []any{"TestRedisJob", 1},
		},
	}).OnQueue("chain").Dispatch())

	time.Sleep(2 * time.Second)
	s.Equal(1, testChainRedisJob)
	s.Equal(1, testRedisJob)
}

type TestRedisJob struct {
}

// Signature The name and signature of the job.
func (receiver *TestRedisJob) Signature() string {
	return "test_redis_job"
}

// Handle Execute the job.
func (receiver *TestRedisJob) Handle(args ...any) error {
	testRedisJob++

	return nil
}

type TestDelayRedisJob struct {
}

// Signature The name and signature of the job.
func (receiver *TestDelayRedisJob) Signature() string {
	return "test_delay_redis_job"
}

// Handle Execute the job.
func (receiver *TestDelayRedisJob) Handle(args ...any) error {
	testDelayRedisJob++

	return nil
}

type TestCustomRedisJob struct {
}

// Signature The name and signature of the job.
func (receiver *TestCustomRedisJob) Signature() string {
	return "test_custom_redis_job"
}

// Handle Execute the job.
func (receiver *TestCustomRedisJob) Handle(args ...any) error {
	testCustomRedisJob++

	return nil
}

type TestErrorRedisJob struct {
}

// Signature The name and signature of the job.
func (receiver *TestErrorRedisJob) Signature() string {
	return "test_error_redis_job"
}

// Handle Execute the job.
func (receiver *TestErrorRedisJob) Handle(args ...any) error {
	testErrorRedisJob++

	return nil
}

type TestChainRedisJob struct {
}

// Signature The name and signature of the job.
func (receiver *TestChainRedisJob) Signature() string {
	return "test_chain_redis_job"
}

// Handle Execute the job.
func (receiver *TestChainRedisJob) Handle(args ...any) error {
	testChainRedisJob++

	return nil
}
