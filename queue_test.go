package redis

import (
	"context"
	"log"
	"testing"
	"time"

	contractsqueue "github.com/goravel/framework/contracts/queue"
	configmock "github.com/goravel/framework/mocks/config"
	ormmock "github.com/goravel/framework/mocks/database/orm"
	queuemock "github.com/goravel/framework/mocks/queue"
	"github.com/goravel/framework/queue"
	"github.com/ory/dockertest/v3"
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
	mockQueue   *queuemock.Queue
	redis       *Queue
	redisDocker *dockertest.Resource
}

func TestQueueTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping tests of using docker")
	}

	redisPool, redisDocker, redisQueue, err := getQueueDocker()
	if err != nil {
		log.Fatalf("Get redis store error: %s", err)
	}

	suite.Run(t, &QueueTestSuite{
		redisDocker: redisDocker,
		redis:       redisQueue,
	})

	if err := redisPool.Purge(redisDocker); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}
}

func (s *QueueTestSuite) SetupTest() {
	s.mockConfig = &configmock.Config{}
	s.mockQueue = &queuemock.Queue{}
	s.app = queue.NewApplication(s.mockConfig)

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

	s.Nil(s.app.Register([]contractsqueue.Job{&TestRedisJob{}, &TestDelayRedisJob{}, &TestCustomRedisJob{}, &TestErrorRedisJob{}, &TestChainRedisJob{}}))
}

func (s *QueueTestSuite) TestDefaultRedisQueue() {
	s.mockConfig.On("GetString", "queue.default").Return("redis").Times(6)
	s.mockConfig.On("GetString", "app.name").Return("goravel").Times(2)
	s.mockConfig.On("GetString", "queue.connections.redis.queue", "default").Return("default").Times(2)
	s.mockConfig.On("GetString", "queue.connections.redis.driver").Return("custom").Times(3)
	s.mockConfig.On("Get", "queue.connections.redis.via").Return(func() (contractsqueue.Driver, error) { return s.redis, nil }).Times(3)
	s.mockConfig.On("GetString", "queue.connections.redis.database").Return("default").Times(2)
	s.mockConfig.On("GetString", "database.redis.default.host").Return("localhost").Times(2)
	s.mockConfig.On("GetString", "database.redis.default.password").Return("").Times(2)
	s.mockConfig.On("GetInt", "database.redis.default.port").Return(s.redisDocker.GetPort("6379/tcp")).Times(2)
	s.mockConfig.On("GetInt", "database.redis.default.database").Return(0).Times(2)
	s.mockConfig.On("GetString", "queue.failed.database").Return("database").Once()
	s.mockConfig.On("GetString", "queue.failed.table").Return("failed_jobs").Once()
	s.mockQueue.On("GetJob", "test_redis_job").Return(TestRedisJob{}, nil).Once()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func(ctx context.Context) {
		s.Nil(s.app.Worker(nil).Run())

		for range ctx.Done() {
			return
		}
	}(ctx)
	time.Sleep(2 * time.Second)
	s.Nil(s.app.Job(&TestRedisJob{}, []any{"TestDefaultRedisQueue", 1}).Dispatch())
	time.Sleep(2 * time.Second)
	s.Equal(1, testRedisJob)

	s.mockConfig.AssertExpectations(s.T())
	s.mockQueue.AssertExpectations(s.T())
}

func (s *QueueTestSuite) TestDelayRedisQueue() {
	s.mockConfig.On("GetString", "queue.default").Return("redis").Times(6)
	s.mockConfig.On("GetString", "app.name").Return("goravel").Times(3)
	s.mockConfig.On("GetString", "queue.connections.redis.queue", "default").Return("default").Once()
	s.mockConfig.On("GetString", "queue.connections.redis.driver").Return("custom").Times(3)
	s.mockConfig.On("Get", "queue.connections.redis.via").Return(func() (contractsqueue.Driver, error) { return s.redis, nil }).Times(3)
	s.mockConfig.On("GetString", "queue.connections.redis.database").Return("default").Times(2)
	s.mockConfig.On("GetString", "database.redis.default.host").Return("localhost").Times(2)
	s.mockConfig.On("GetString", "database.redis.default.password").Return("").Times(2)
	s.mockConfig.On("GetInt", "database.redis.default.port").Return(s.redisDocker.GetPort("6379/tcp")).Times(2)
	s.mockConfig.On("GetInt", "database.redis.default.database").Return(0).Times(2)
	s.mockConfig.On("GetString", "queue.failed.database").Return("database").Once()
	s.mockConfig.On("GetString", "queue.failed.table").Return("failed_jobs").Once()
	s.mockQueue.On("GetJob", "test_delay_redis_job").Return(TestDelayRedisJob{}, nil).Once()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func(ctx context.Context) {
		s.Nil(s.app.Worker(&contractsqueue.Args{
			Queue: "delay",
		}).Run())

		for range ctx.Done() {
			return
		}
	}(ctx)
	time.Sleep(2 * time.Second)
	s.Nil(s.app.Job(&TestDelayRedisJob{}, []any{"TestDelayRedisQueue", 1}).OnQueue("delay").Delay(3).Dispatch())
	time.Sleep(2 * time.Second)
	s.Equal(0, testDelayRedisJob)
	time.Sleep(3 * time.Second)
	s.Equal(1, testDelayRedisJob)

	s.mockConfig.AssertExpectations(s.T())
	s.mockQueue.AssertExpectations(s.T())
}

func (s *QueueTestSuite) TestCustomRedisQueue() {
	s.mockConfig.On("GetString", "queue.default").Return("custom").Times(7)
	s.mockConfig.On("GetString", "app.name").Return("goravel").Times(3)
	s.mockConfig.On("GetString", "queue.connections.custom.queue", "default").Return("default").Once()
	s.mockConfig.On("GetString", "queue.connections.custom.driver").Return("custom").Times(4)
	s.mockConfig.On("Get", "queue.connections.custom.via").Return(func() (contractsqueue.Driver, error) { return s.redis, nil }).Times(3)
	s.mockConfig.On("GetString", "queue.connections.custom.database").Return("default").Times(3)
	s.mockConfig.On("GetString", "database.redis.default.host").Return("localhost").Times(3)
	s.mockConfig.On("GetString", "database.redis.default.password").Return("").Times(3)
	s.mockConfig.On("GetInt", "database.redis.default.port").Return(s.redisDocker.GetPort("6379/tcp")).Times(3)
	s.mockConfig.On("GetInt", "database.redis.default.database").Return(0).Times(3)
	s.mockConfig.On("GetString", "queue.failed.database").Return("database").Once()
	s.mockConfig.On("GetString", "queue.failed.table").Return("failed_jobs").Once()
	s.mockQueue.On("GetJob", "test_custom_redis_job").Return(TestCustomRedisJob{}, nil).Once()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func(ctx context.Context) {
		s.Nil(s.app.Worker(&contractsqueue.Args{
			Connection: "custom",
			Queue:      "custom1",
			Concurrent: 2,
		}).Run())

		for range ctx.Done() {
			return
		}
	}(ctx)
	time.Sleep(2 * time.Second)
	s.Nil(s.app.Job(&TestCustomRedisJob{}, []any{"TestCustomRedisQueue", 1}).OnConnection("custom").OnQueue("custom1").Dispatch())
	time.Sleep(2 * time.Second)
	s.Equal(1, testCustomRedisJob)

	s.mockConfig.AssertExpectations(s.T())
	s.mockQueue.AssertExpectations(s.T())
}

func (s *QueueTestSuite) TestErrorRedisQueue() {
	s.mockConfig.On("GetString", "queue.default").Return("redis").Times(7)
	s.mockConfig.On("GetString", "app.name").Return("goravel").Times(3)
	s.mockConfig.On("GetString", "queue.connections.redis.queue", "default").Return("default").Once()
	s.mockConfig.On("GetString", "queue.connections.redis.driver").Return("custom").Times(4)
	s.mockConfig.On("Get", "queue.connections.redis.via").Return(func() (contractsqueue.Driver, error) { return s.redis, nil }).Times(3)
	s.mockConfig.On("GetString", "queue.connections.redis.database").Return("default").Times(3)
	s.mockConfig.On("GetString", "database.redis.default.host").Return("localhost").Times(3)
	s.mockConfig.On("GetString", "database.redis.default.password").Return("").Times(3)
	s.mockConfig.On("GetInt", "database.redis.default.port").Return(s.redisDocker.GetPort("6379/tcp")).Times(3)
	s.mockConfig.On("GetInt", "database.redis.default.database").Return(0).Times(3)
	s.mockConfig.On("GetString", "queue.failed.database").Return("database").Once()
	s.mockConfig.On("GetString", "queue.failed.table").Return("failed_jobs").Once()
	s.mockQueue.On("GetJob", "test_error_redis_job").Return(TestErrorRedisJob{}, nil).Once()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func(ctx context.Context) {
		s.Nil(s.app.Worker(&contractsqueue.Args{
			Queue: "error",
		}).Run())

		for range ctx.Done() {
			return
		}
	}(ctx)
	time.Sleep(2 * time.Second)
	s.Nil(s.app.Job(&TestErrorRedisJob{}, []any{"TestErrorRedisQueue", 1}).OnConnection("redis").OnQueue("error1").Dispatch())
	time.Sleep(2 * time.Second)
	s.Equal(0, testErrorRedisJob)

	s.mockConfig.AssertExpectations(s.T())
	s.mockQueue.AssertExpectations(s.T())
}

func (s *QueueTestSuite) TestChainRedisQueue() {
	s.mockConfig.On("GetString", "queue.default").Return("redis").Times(6)
	s.mockConfig.On("GetString", "app.name").Return("goravel").Times(3)
	s.mockConfig.On("GetString", "queue.connections.redis.queue", "default").Return("default").Once()
	s.mockConfig.On("GetString", "queue.connections.redis.driver").Return("custom").Times(3)
	s.mockConfig.On("Get", "queue.connections.redis.via").Return(func() (contractsqueue.Driver, error) { return s.redis, nil }).Times(3)
	s.mockConfig.On("GetString", "queue.connections.redis.database").Return("default").Times(2)
	s.mockConfig.On("GetString", "database.redis.default.host").Return("localhost").Times(2)
	s.mockConfig.On("GetString", "database.redis.default.password").Return("").Times(2)
	s.mockConfig.On("GetInt", "database.redis.default.port").Return(s.redisDocker.GetPort("6379/tcp")).Times(2)
	s.mockConfig.On("GetInt", "database.redis.default.database").Return(0).Times(2)
	s.mockConfig.On("GetString", "queue.failed.database").Return("database").Once()
	s.mockConfig.On("GetString", "queue.failed.table").Return("failed_jobs").Once()
	s.mockQueue.On("GetJob", "test_chain_redis_job").Return(TestChainRedisJob{}, nil).Once()
	s.mockQueue.On("GetJob", "test_redis_job").Return(TestRedisJob{}, nil).Once()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func(ctx context.Context) {
		s.Nil(s.app.Worker(&contractsqueue.Args{
			Queue: "chain",
		}).Run())

		for range ctx.Done() {
			return
		}
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
