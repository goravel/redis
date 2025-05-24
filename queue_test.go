package redis

import (
	"context"
	"testing"
	"time"

	contractsqueue "github.com/goravel/framework/contracts/queue"
	"github.com/goravel/framework/errors"
	"github.com/goravel/framework/foundation/json"
	mocksconfig "github.com/goravel/framework/mocks/config"
	mocksqueue "github.com/goravel/framework/mocks/queue"
	"github.com/goravel/framework/support/docker"
	"github.com/goravel/framework/support/env"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
	"github.com/stretchr/testify/suite"
)

type QueueTestSuite struct {
	suite.Suite
	mockQueue   *mocksqueue.Queue
	queue       *Queue
	redisDocker *docker.Redis
}

func TestQueueTestSuite(t *testing.T) {
	if env.IsWindows() {
		t.Skip("Skipping tests of using docker")
	}

	suite.Run(t, &QueueTestSuite{})
}

func (s *QueueTestSuite) SetupSuite() {
	redisDocker := docker.NewRedis()
	s.Nil(redisDocker.Build())
	s.redisDocker = redisDocker

	mockConfig := mocksconfig.NewConfig(s.T())
	mockConfig.EXPECT().GetString("app.name", "goravel").Return("test").Once()
	mockConfig.EXPECT().GetString("queue.connections.redis.connection", "default").Return("queue-default").Once()
	mockConfig.EXPECT().GetString("database.redis.queue-default.host").Return("localhost").Once()
	mockConfig.EXPECT().GetString("database.redis.queue-default.port", "6379").Return(cast.ToString(redisDocker.Config().Port)).Once()
	mockConfig.EXPECT().GetString("database.redis.queue-default.username").Return("").Once()
	mockConfig.EXPECT().GetString("database.redis.queue-default.password").Return("").Once()
	mockConfig.EXPECT().GetInt("database.redis.queue-default.database", 0).Return(0).Once()
	mockConfig.EXPECT().Get("database.redis.queue-default.tls").Return(nil).Once()

	mockQueue := mocksqueue.NewQueue(s.T())
	s.mockQueue = mockQueue

	queue, err := NewQueue(context.Background(), mockConfig, mockQueue, json.New(), "redis")
	s.Nil(err)
	s.queue = queue
}

func (s *QueueTestSuite) TearDownSuite() {
	s.NoError(s.redisDocker.Shutdown())
}

func (s *QueueTestSuite) SetupTest() {

}

func (s *QueueTestSuite) TestQueueKey() {
	s.Equal("test_queues:redis_test-queue", s.queue.queueKey("test-queue"))
}

func (s *QueueTestSuite) TestDelayQueueKey() {
	s.Equal("test_queues:redis_test-queue:delayed", s.queue.delayQueueKey("test-queue"))
}

func (s *QueueTestSuite) TestDriver() {
	s.Equal("custom", s.queue.Driver())
}

func (s *QueueTestSuite) TestPush() {
	s.Run("no delay", func() {
		queue := "no-delay"
		queueKey := s.queue.queueKey(queue)
		task := contractsqueue.Task{
			UUID: "865111de-ff50-4652-9733-72fea655f836",
			Jobs: contractsqueue.Jobs{
				Job:  &MockJob{},
				Args: testArgs,
			},
			Chain: []contractsqueue.Jobs{
				{
					Job:   &MockJob{},
					Args:  testArgs,
					Delay: time.Now().Add(1 * time.Hour),
				},
			},
		}

		s.NoError(s.queue.Push(task, queue))

		count, err := s.queue.instance.LLen(context.Background(), queueKey).Result()
		s.NoError(err)
		s.Equal(int64(1), count)

		count, err = s.queue.instance.LLen(context.Background(), s.queue.delayQueueKey(queueKey)).Result()
		s.NoError(err)
		s.Equal(int64(0), count)
	})

	s.Run("delay", func() {
		queue := "delay"
		queueKey := s.queue.queueKey(queue)
		task := contractsqueue.Task{
			UUID: "865111de-ff50-4652-9733-72fea655f836",
			Jobs: contractsqueue.Jobs{
				Job:   &MockJob{},
				Args:  testArgs,
				Delay: time.Now().Add(2 * time.Second),
			},
			Chain: []contractsqueue.Jobs{
				{
					Job:   &MockJob{},
					Args:  testArgs,
					Delay: time.Now().Add(1 * time.Hour),
				},
			},
		}

		s.NoError(s.queue.Push(task, queue))

		count, err := s.queue.instance.LLen(context.Background(), queueKey).Result()
		s.NoError(err)
		s.Equal(int64(0), count)

		count, err = s.queue.instance.ZCount(context.Background(), s.queue.delayQueueKey(queue), "-inf", "+inf").Result()
		s.NoError(err)
		s.Equal(int64(1), count)
	})
}

func (s *QueueTestSuite) TestPop() {
	s.Run("no job", func() {
		queue := "no-job"
		task, err := s.queue.Pop(queue)
		s.Equal(errors.QueueDriverNoJobFound.Args(s.queue.queueKey(queue)), err)
		s.Equal(contractsqueue.Task{}, task)
	})

	s.Run("success", func() {
		queue := "pop"
		queueKey := s.queue.queueKey(queue)
		task := contractsqueue.Task{
			UUID: "865111de-ff50-4652-9733-72fea655f836",
			Jobs: contractsqueue.Jobs{
				Job:  &MockJob{},
				Args: testArgs,
			},
			Chain: []contractsqueue.Jobs{
				{
					Job:   &MockJob{},
					Args:  testArgs,
					Delay: time.Now().Add(1 * time.Hour),
				},
			},
		}

		s.NoError(s.queue.Push(task, queue))

		s.mockQueue.EXPECT().GetJob(task.Job.Signature()).Return(&MockJob{}, nil).Twice()
		task1, err := s.queue.Pop(queue)

		s.NoError(err)
		s.Equal(task.Job.Signature(), task1.Job.Signature())
		s.Equal(len(task.Args), len(task1.Args))
		for i, arg := range task1.Args {
			s.Equal(task.Args[i].Type, arg.Type)
		}
		s.Equal(task.Delay, task1.Delay)
		s.Len(task1.Chain, 1)
		for i, chained := range task1.Chain {
			s.Equal(task.Chain[i].Job.Signature(), chained.Job.Signature())
			s.Equal(len(task.Chain[i].Args), len(chained.Args))
			for j, arg := range chained.Args {
				s.Equal(task.Chain[i].Args[j].Type, arg.Type)
			}
			s.Equal(task.Chain[i].Delay.Format(time.RFC3339), chained.Delay.Format(time.RFC3339))
		}

		count, err := s.queue.instance.LLen(context.Background(), queueKey).Result()
		s.NoError(err)
		s.Equal(int64(0), count)
	})
}

func (s *QueueTestSuite) TestLater() {
	queue := "later"
	queueKey := s.queue.queueKey(queue)
	task := contractsqueue.Task{
		UUID: "865111de-ff50-4652-9733-72fea655f836",
		Jobs: contractsqueue.Jobs{
			Job:   &MockJob{},
			Args:  testArgs,
			Delay: time.Now().Add(1 * time.Second),
		},
		Chain: []contractsqueue.Jobs{
			{
				Job:   &MockJob{},
				Args:  testArgs,
				Delay: time.Now().Add(1 * time.Hour),
			},
		},
	}

	s.NoError(s.queue.Push(task, queue))

	count, err := s.queue.instance.LLen(context.Background(), queueKey).Result()
	s.NoError(err)
	s.Equal(int64(0), count)

	count, err = s.queue.instance.ZCount(context.Background(), s.queue.delayQueueKey(queue), "-inf", "+inf").Result()
	s.NoError(err)
	s.Equal(int64(1), count)

	task1, err := s.queue.Pop(queue)

	s.Equal(errors.QueueDriverNoJobFound.Args(queueKey), err)
	s.Equal(contractsqueue.Task{}, task1)

	time.Sleep(1 * time.Second)

	s.mockQueue.EXPECT().GetJob(task.Job.Signature()).Return(&MockJob{}, nil).Twice()
	task1, err = s.queue.Pop(queue)
	s.NoError(err)
	s.NotNil(task1)
	s.Equal(task.Job.Signature(), task1.Job.Signature())

	count, err = s.queue.instance.LLen(context.Background(), queueKey).Result()
	s.NoError(err)
	s.Equal(int64(0), count)

	count, err = s.queue.instance.ZCount(context.Background(), s.queue.delayQueueKey(queue), "-inf", "+inf").Result()
	s.NoError(err)
	s.Equal(int64(0), count)
}

func (s *QueueTestSuite) TestMigrateDelayedJobs() {
	// Add a delayed job
	queue := "test-queue"
	delay := time.Now().Add(-1 * time.Hour) // Past time
	payload := "test-payload"
	s.queue.instance.ZAdd(context.Background(), s.queue.delayQueueKey(queue), redis.Z{
		Score:  float64(delay.Unix()),
		Member: payload,
	})

	err := s.queue.migrateDelayedJobs("test-queue")
	s.Nil(err)

	// Verify the job was moved to the main queue
	result, err := s.queue.instance.LPop(context.Background(), s.queue.queueKey(queue)).Result()
	s.Nil(err)
	s.Equal(payload, result)
}

var (
	testArgs = []contractsqueue.Arg{
		{
			Type:  "bool",
			Value: true,
		},
		{
			Type:  "int",
			Value: 1,
		},
		{
			Type:  "int8",
			Value: int8(1),
		},
		{
			Type:  "int16",
			Value: int16(1),
		},
		{
			Type:  "int32",
			Value: int32(1),
		},
		{
			Type:  "int64",
			Value: int64(1),
		},
		{
			Type:  "uint",
			Value: uint(1),
		},
		{
			Type:  "uint8",
			Value: uint8(1),
		},
		{
			Type:  "uint16",
			Value: uint16(1),
		},
		{
			Type:  "uint32",
			Value: uint32(1),
		},
		{
			Type:  "uint64",
			Value: uint64(1),
		},
		{
			Type:  "float32",
			Value: float32(1.1),
		},
		{
			Type:  "float64",
			Value: float64(1.2),
		},
		{
			Type:  "string",
			Value: "test",
		},
		{
			Type:  "[]bool",
			Value: []bool{true, false},
		},
		{
			Type:  "[]int",
			Value: []int{1, 2, 3},
		},
		{
			Type:  "[]int8",
			Value: []int8{1, 2, 3},
		},
		{
			Type:  "[]int16",
			Value: []int16{1, 2, 3},
		},
		{
			Type:  "[]int32",
			Value: []int32{1, 2, 3},
		},
		{
			Type:  "[]int64",
			Value: []int64{1, 2, 3},
		},
		{
			Type:  "[]uint",
			Value: []uint{1, 2, 3},
		},
		{
			Type:  "[]uint8",
			Value: []uint8{1, 2, 3},
		},
		{
			Type:  "[]uint16",
			Value: []uint16{1, 2, 3},
		},
		{
			Type:  "[]uint32",
			Value: []uint32{1, 2, 3},
		},
		{
			Type:  "[]uint64",
			Value: []uint64{1, 2, 3},
		},
		{
			Type:  "[]float32",
			Value: []float32{1.1, 1.2, 1.3},
		},
		{
			Type:  "[]float64",
			Value: []float64{1.1, 1.2, 1.3},
		},
		{
			Type:  "[]string",
			Value: []string{"test", "test2", "test3"},
		},
	}
)

type MockJob struct {
}

func (m *MockJob) Signature() string {
	return "mock"
}

func (m *MockJob) Handle(args ...any) error {
	return nil
}
