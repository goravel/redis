package redis

import (
	"context"
	"testing"
	"time"

	"github.com/spf13/cast"
	"github.com/stretchr/testify/assert"

	contractsqueue "github.com/goravel/framework/contracts/queue"
	configmock "github.com/goravel/framework/mocks/config"
	ormmock "github.com/goravel/framework/mocks/database/orm"
	"github.com/goravel/framework/queue"
	testingdocker "github.com/goravel/framework/support/docker"
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
	redisDocker *testingdocker.Redis
}

func TestQueueTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping tests of using docker")
	}

	redisDocker := testingdocker.NewRedis()
	assert.Nil(t, redisDocker.Build())

	mockConfig := &configmock.Config{}
	mockConfig.EXPECT().GetString("queue.connections.redis.connection", "default").Return("default").Once()
	mockConfig.EXPECT().GetString("database.redis.default.host").Return("localhost").Once()
	mockConfig.EXPECT().GetString("database.redis.default.port").Return(cast.ToString(redisDocker.Config().Port)).Once()
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

	mockOrm := &ormmock.Orm{}
	mockQuery := &ormmock.Query{}
	mockOrm.EXPECT().Connection("database").Return(mockOrm)
	mockOrm.EXPECT().Query().Return(mockQuery)
	mockQuery.EXPECT().Table("failed_jobs").Return(mockQuery)

	queue.OrmFacade = mockOrm
}

func (s *QueueTestSuite) TestDefaultRedisQueue() {
	s.mockConfig.EXPECT().GetString("queue.default").Return("redis")
	s.mockConfig.EXPECT().GetString("app.name").Return("goravel")
	s.mockConfig.EXPECT().GetString("queue.connections.redis.queue", "default").Return("default")
	s.mockConfig.EXPECT().GetString("queue.connections.redis.driver").Return("custom")
	s.mockConfig.EXPECT().Get("queue.connections.redis.via").Return(func() (contractsqueue.Driver, error) { return s.redis, nil })
	s.mockConfig.EXPECT().GetString("database.redis.default.host").Return("localhost")
	s.mockConfig.EXPECT().GetString("database.redis.default.password").Return("")
	s.mockConfig.EXPECT().GetInt("database.redis.default.database").Return(0)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func(ctx context.Context) {
		worker := s.app.Worker()
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
	s.mockConfig.EXPECT().GetString("queue.default").Return("redis")
	s.mockConfig.EXPECT().GetString("app.name").Return("goravel")
	s.mockConfig.EXPECT().GetString("queue.connections.redis.queue", "default").Return("default")
	s.mockConfig.EXPECT().GetString("queue.connections.redis.driver").Return("custom")
	s.mockConfig.EXPECT().Get("queue.connections.redis.via").Return(func() (contractsqueue.Driver, error) { return s.redis, nil })
	s.mockConfig.EXPECT().GetString("database.redis.default.host").Return("localhost")
	s.mockConfig.EXPECT().GetString("database.redis.default.password").Return("")
	s.mockConfig.EXPECT().GetInt("database.redis.default.database").Return(0)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func(ctx context.Context) {
		worker := s.app.Worker(contractsqueue.Args{
			Queue: "delay",
		})
		s.Nil(worker.Run())

		<-ctx.Done()
		s.Nil(worker.Shutdown())
	}(ctx)
	time.Sleep(2 * time.Second)
	s.Nil(s.app.Job(&TestDelayRedisJob{}, []any{"TestDelayRedisQueue", 1}).OnQueue("delay").Delay(time.Now().Add(3 * time.Second)).Dispatch())
	time.Sleep(2 * time.Second)
	s.Equal(0, testDelayRedisJob)
	time.Sleep(3 * time.Second)
	s.Equal(1, testDelayRedisJob)

	s.mockConfig.AssertExpectations(s.T())
}

func (s *QueueTestSuite) TestCustomRedisQueue() {
	s.mockConfig.EXPECT().GetString("queue.default").Return("custom")
	s.mockConfig.EXPECT().GetString("app.name").Return("goravel")
	s.mockConfig.EXPECT().GetString("queue.connections.custom.driver").Return("custom")
	s.mockConfig.EXPECT().Get("queue.connections.custom.via").Return(func() (contractsqueue.Driver, error) { return s.redis, nil })
	s.mockConfig.EXPECT().GetString("database.redis.default.host").Return("localhost")
	s.mockConfig.EXPECT().GetString("database.redis.default.password").Return("")
	s.mockConfig.EXPECT().GetInt("database.redis.default.database").Return(0)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func(ctx context.Context) {
		worker := s.app.Worker(contractsqueue.Args{
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
	s.mockConfig.EXPECT().GetString("queue.default").Return("redis")
	s.mockConfig.EXPECT().GetString("app.name").Return("goravel")
	s.mockConfig.EXPECT().GetString("queue.connections.redis.queue", "default").Return("default")
	s.mockConfig.EXPECT().GetString("queue.connections.redis.driver").Return("custom")
	s.mockConfig.EXPECT().Get("queue.connections.redis.via").Return(func() (contractsqueue.Driver, error) { return s.redis, nil })
	s.mockConfig.EXPECT().GetString("database.redis.default.host").Return("localhost")
	s.mockConfig.EXPECT().GetString("database.redis.default.password").Return("")
	s.mockConfig.EXPECT().GetInt("database.redis.default.database").Return(0)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func(ctx context.Context) {
		worker := s.app.Worker(contractsqueue.Args{
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
	s.mockConfig.EXPECT().GetString("queue.default").Return("redis")
	s.mockConfig.EXPECT().GetString("app.name").Return("goravel")
	s.mockConfig.EXPECT().GetString("queue.connections.redis.queue", "default").Return("default")
	s.mockConfig.EXPECT().GetString("queue.connections.redis.driver").Return("custom")
	s.mockConfig.EXPECT().Get("queue.connections.redis.via").Return(func() (contractsqueue.Driver, error) { return s.redis, nil })
	s.mockConfig.EXPECT().GetString("database.redis.default.host").Return("localhost")
	s.mockConfig.EXPECT().GetString("database.redis.default.password").Return("")
	s.mockConfig.EXPECT().GetInt("database.redis.default.database").Return(0)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func(ctx context.Context) {
		worker := s.app.Worker(contractsqueue.Args{
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
