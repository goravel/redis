package redis

import (
	"context"
	"log"
	"testing"
	"time"

	contractsqueue "github.com/goravel/framework/contracts/queue"
	configmock "github.com/goravel/framework/mocks/config"
	ormmock "github.com/goravel/framework/mocks/database/orm"
	"github.com/goravel/framework/queue"
	"github.com/ory/dockertest/v3"
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
	mockConfig  *configmock.Config
	redis       *Queue
	redisDocker *dockertest.Resource
}

func TestQueueTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping tests of using docker")
	}

	mockConfig := &configmock.Config{}
	app := queue.NewApplication(mockConfig)
	redisPool, redisDocker, redisQueue, err := getQueueDocker(mockConfig, app)
	if err != nil {
		log.Fatalf("Get redis store error: %s", err)
	}
	assert.Nil(t, app.Register([]contractsqueue.Job{&TestRedisJob{}, &TestDelayRedisJob{}, &TestCustomRedisJob{}, &TestErrorRedisJob{}, &TestChainRedisJob{}}))

	suite.Run(t, &QueueTestSuite{
		app:         app,
		mockConfig:  mockConfig,
		redisDocker: redisDocker,
		redis:       redisQueue,
	})

	if err = redisPool.Purge(redisDocker); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}
}

func (s *QueueTestSuite) SetupTest() {
	testRedisJob = 0
	testDelayRedisJob = 0
	testCustomRedisJob = 0
	testErrorRedisJob = 0
	testChainRedisJob = 0

	mockOrm := &ormmock.Orm{}
	mockQuery := &ormmock.Query{}
	mockOrm.On("Connection", "database").Return(mockOrm)
	mockOrm.On("Query").Return(mockQuery)
	mockQuery.On("Table", "failed_jobs").Return(mockQuery)

	queue.OrmFacade = mockOrm
}

func (s *QueueTestSuite) TestDefaultRedisQueue() {
	s.mockConfig.On("GetString", "queue.default").Return("redis")
	s.mockConfig.On("GetString", "app.name").Return("goravel")
	s.mockConfig.On("GetString", "queue.connections.redis.queue", "default").Return("default")
	s.mockConfig.On("GetString", "queue.connections.redis.driver").Return("custom")
	s.mockConfig.On("Get", "queue.connections.redis.via").Return(func() (contractsqueue.Driver, error) { return s.redis, nil })
	s.mockConfig.On("GetString", "database.redis.default.host").Return("localhost")
	s.mockConfig.On("GetString", "database.redis.default.password").Return("")
	s.mockConfig.On("GetInt", "database.redis.default.database").Return(0)
	s.mockConfig.On("GetString", "queue.failed.database").Return("database")
	s.mockConfig.On("GetString", "queue.failed.table").Return("failed_jobs")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func(ctx context.Context) {
		worker := s.app.Worker(nil)
		s.Nil(worker.Run())

		<-ctx.Done()
		s.Nil(worker.Shutdown())
	}(ctx)
	time.Sleep(2 * time.Second)
	s.Nil(s.app.Job(&TestRedisJob{}, []any{"TestDefaultRedisQueue", 1}).Dispatch())
	time.Sleep(2 * time.Second)
	s.Equal(1, testRedisJob)

	s.mockConfig.AssertExpectations(s.T())
}

func (s *QueueTestSuite) TestDelayRedisQueue() {
	s.mockConfig.On("GetString", "queue.default").Return("redis")
	s.mockConfig.On("GetString", "app.name").Return("goravel")
	s.mockConfig.On("GetString", "queue.connections.redis.queue", "default").Return("default")
	s.mockConfig.On("GetString", "queue.connections.redis.driver").Return("custom")
	s.mockConfig.On("Get", "queue.connections.redis.via").Return(func() (contractsqueue.Driver, error) { return s.redis, nil })
	s.mockConfig.On("GetString", "database.redis.default.host").Return("localhost")
	s.mockConfig.On("GetString", "database.redis.default.password").Return("")
	s.mockConfig.On("GetInt", "database.redis.default.database").Return(0)
	s.mockConfig.On("GetString", "queue.failed.database").Return("database")
	s.mockConfig.On("GetString", "queue.failed.table").Return("failed_jobs")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func(ctx context.Context) {
		worker := s.app.Worker(&contractsqueue.Args{
			Queue: "delay",
		})
		s.Nil(worker.Run())

		<-ctx.Done()
		s.Nil(worker.Shutdown())
	}(ctx)
	time.Sleep(2 * time.Second)
	s.Nil(s.app.Job(&TestDelayRedisJob{}, []any{"TestDelayRedisQueue", 1}).OnQueue("delay").Delay(3).Dispatch())
	time.Sleep(2 * time.Second)
	s.Equal(0, testDelayRedisJob)
	time.Sleep(3 * time.Second)
	s.Equal(1, testDelayRedisJob)

	s.mockConfig.AssertExpectations(s.T())
}

func (s *QueueTestSuite) TestCustomRedisQueue() {
	s.mockConfig.On("GetString", "queue.default").Return("custom")
	s.mockConfig.On("GetString", "app.name").Return("goravel")
	s.mockConfig.On("GetString", "queue.connections.custom.driver").Return("custom")
	s.mockConfig.On("Get", "queue.connections.custom.via").Return(func() (contractsqueue.Driver, error) { return s.redis, nil })
	s.mockConfig.On("GetString", "database.redis.default.host").Return("localhost")
	s.mockConfig.On("GetString", "database.redis.default.password").Return("")
	s.mockConfig.On("GetInt", "database.redis.default.database").Return(0)
	s.mockConfig.On("GetString", "queue.failed.database").Return("database")
	s.mockConfig.On("GetString", "queue.failed.table").Return("failed_jobs")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func(ctx context.Context) {
		worker := s.app.Worker(&contractsqueue.Args{
			Connection: "custom",
			Queue:      "custom1",
			Concurrent: 2,
		})
		s.Nil(worker.Run())

		<-ctx.Done()
		s.Nil(worker.Shutdown())
	}(ctx)
	time.Sleep(2 * time.Second)
	s.Nil(s.app.Job(&TestCustomRedisJob{}, []any{"TestCustomRedisQueue", 1}).OnConnection("custom").OnQueue("custom1").Dispatch())
	time.Sleep(2 * time.Second)
	s.Equal(1, testCustomRedisJob)

	s.mockConfig.AssertExpectations(s.T())
}

func (s *QueueTestSuite) TestErrorRedisQueue() {
	s.mockConfig.On("GetString", "queue.default").Return("redis")
	s.mockConfig.On("GetString", "app.name").Return("goravel")
	s.mockConfig.On("GetString", "queue.connections.redis.queue", "default").Return("default")
	s.mockConfig.On("GetString", "queue.connections.redis.driver").Return("custom")
	s.mockConfig.On("Get", "queue.connections.redis.via").Return(func() (contractsqueue.Driver, error) { return s.redis, nil })
	s.mockConfig.On("GetString", "database.redis.default.host").Return("localhost")
	s.mockConfig.On("GetString", "database.redis.default.password").Return("")
	s.mockConfig.On("GetInt", "database.redis.default.database").Return(0)
	s.mockConfig.On("GetString", "queue.failed.database").Return("database")
	s.mockConfig.On("GetString", "queue.failed.table").Return("failed_jobs")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func(ctx context.Context) {
		worker := s.app.Worker(&contractsqueue.Args{
			Queue: "error",
		})
		s.Nil(worker.Run())

		<-ctx.Done()
		s.Nil(worker.Shutdown())
	}(ctx)
	time.Sleep(2 * time.Second)
	s.Nil(s.app.Job(&TestErrorRedisJob{}, []any{"TestErrorRedisQueue", 1}).OnConnection("redis").OnQueue("error1").Dispatch())
	time.Sleep(2 * time.Second)
	s.Equal(0, testErrorRedisJob)

	s.mockConfig.AssertExpectations(s.T())
}

func (s *QueueTestSuite) TestChainRedisQueue() {
	s.mockConfig.On("GetString", "queue.default").Return("redis")
	s.mockConfig.On("GetString", "app.name").Return("goravel")
	s.mockConfig.On("GetString", "queue.connections.redis.queue", "default").Return("default")
	s.mockConfig.On("GetString", "queue.connections.redis.driver").Return("custom")
	s.mockConfig.On("Get", "queue.connections.redis.via").Return(func() (contractsqueue.Driver, error) { return s.redis, nil })
	s.mockConfig.On("GetString", "database.redis.default.host").Return("localhost")
	s.mockConfig.On("GetString", "database.redis.default.password").Return("")
	s.mockConfig.On("GetInt", "database.redis.default.database").Return(0)
	s.mockConfig.On("GetString", "queue.failed.database").Return("database")
	s.mockConfig.On("GetString", "queue.failed.table").Return("failed_jobs")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func(ctx context.Context) {
		worker := s.app.Worker(&contractsqueue.Args{
			Queue: "chain",
		})
		s.Nil(worker.Run())

		<-ctx.Done()
		s.Nil(worker.Shutdown())
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

	s.mockConfig.AssertExpectations(s.T())
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
