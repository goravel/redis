package redis

import (
	"context"
	"testing"
	"time"

	"github.com/goravel/framework/contracts/queue"
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
	mockConfig.EXPECT().GetString("queue.connections.redis.connection", "default").Return("default").Once()
	mockConfig.EXPECT().GetString("database.redis.default.host").Return("localhost").Once()
	mockConfig.EXPECT().GetString("database.redis.default.port").Return(cast.ToString(redisDocker.Config().Port)).Once()
	mockConfig.EXPECT().GetString("database.redis.default.username").Return("").Once()
	mockConfig.EXPECT().GetString("database.redis.default.password").Return("").Once()
	mockConfig.EXPECT().GetInt("database.redis.default.database").Return(0).Once()

	mockQueue := mocksqueue.NewQueue(s.T())
	s.mockQueue = mockQueue

	queue, err := NewQueue(context.Background(), mockConfig, mockQueue, json.NewJson(), "redis")
	s.Nil(err)
	s.queue = queue
}

func (s *QueueTestSuite) TearDownSuite() {
	s.Nil(s.redisDocker.Shutdown())
}

func (s *QueueTestSuite) SetupTest() {

}

func (s *QueueTestSuite) TestConnection() {
	s.Equal("default", s.queue.Connection())
}

func (s *QueueTestSuite) TestDelayQueueKey() {
	s.Equal("test-queue:delayed", s.queue.delayQueueKey("test-queue"))
}

func (s *QueueTestSuite) TestDriver() {
	s.Equal("custom", s.queue.Driver())
}

func (s *QueueTestSuite) TestName() {
	s.Equal("redis", s.queue.Name())
}

func (s *QueueTestSuite) TestPush() {
	s.Run("no delay", func() {
		queueKey := "no-delay"
		task := queue.Task{
			Uuid: "865111de-ff50-4652-9733-72fea655f836",
			Jobs: queue.Jobs{
				Job:  &MockJob{},
				Args: mockArgs,
			},
			Chain: []queue.Jobs{
				{
					Job:   &MockJob{},
					Args:  mockArgs,
					Delay: time.Now().Add(1 * time.Hour),
				},
			},
		}

		s.NoError(s.queue.Push(task, queueKey))

		count, err := s.queue.instance.LLen(context.Background(), queueKey).Result()
		s.NoError(err)
		s.Equal(int64(1), count)

		count, err = s.queue.instance.LLen(context.Background(), s.queue.delayQueueKey(queueKey)).Result()
		s.NoError(err)
		s.Equal(int64(0), count)
	})

	s.Run("delay", func() {
		queueKey := "delay"
		task := queue.Task{
			Uuid: "865111de-ff50-4652-9733-72fea655f836",
			Jobs: queue.Jobs{
				Job:   &MockJob{},
				Args:  mockArgs,
				Delay: time.Now().Add(2 * time.Second),
			},
			Chain: []queue.Jobs{
				{
					Job:   &MockJob{},
					Args:  mockArgs,
					Delay: time.Now().Add(1 * time.Hour),
				},
			},
		}

		s.NoError(s.queue.Push(task, queueKey))

		count, err := s.queue.instance.LLen(context.Background(), queueKey).Result()
		s.NoError(err)
		s.Equal(int64(0), count)

		count, err = s.queue.instance.ZCount(context.Background(), s.queue.delayQueueKey(queueKey), "-inf", "+inf").Result()
		s.NoError(err)
		s.Equal(int64(1), count)
	})
}

func (s *QueueTestSuite) TestPop() {
	s.Run("no job", func() {
		queueKey := "no-job"
		task, err := s.queue.Pop(queueKey)
		s.Equal(errors.QueueDriverNoJobFound.Args(queueKey), err)
		s.Nil(task)
	})

	s.Run("success", func() {
		queueKey := "pop"
		task := queue.Task{
			Uuid: "865111de-ff50-4652-9733-72fea655f836",
			Jobs: queue.Jobs{
				Job:  &MockJob{},
				Args: mockArgs,
			},
			Chain: []queue.Jobs{
				{
					Job:   &MockJob{},
					Args:  mockArgs,
					Delay: time.Now().Add(1 * time.Hour),
				},
			},
		}

		s.NoError(s.queue.Push(task, queueKey))

		s.mockQueue.EXPECT().GetJob(task.Job.Signature()).Return(&MockJob{}, nil).Twice()
		task1, err := s.queue.Pop(queueKey)

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
	queueKey := "later"
	task := queue.Task{
		Uuid: "865111de-ff50-4652-9733-72fea655f836",
		Jobs: queue.Jobs{
			Job:  &MockJob{},
			Args: mockArgs,
		},
		Chain: []queue.Jobs{
			{
				Job:   &MockJob{},
				Args:  mockArgs,
				Delay: time.Now().Add(1 * time.Hour),
			},
		},
	}

	s.NoError(s.queue.Push(task, queueKey))

	count, err := s.queue.instance.LLen(context.Background(), queueKey).Result()
	s.NoError(err)
	s.Equal(int64(0), count)

	count, err = s.queue.instance.ZCount(context.Background(), s.queue.delayQueueKey(queueKey), "-inf", "+inf").Result()
	s.NoError(err)
	s.Equal(int64(1), count)

	task1, err := s.queue.Pop(queueKey)

	s.Equal(errors.QueueDriverNoJobFound.Args(queueKey), err)
	s.Nil(task1)

	time.Sleep(3 * time.Second)

	s.mockQueue.EXPECT().GetJob(task.Job.Signature()).Return(&MockJob{}, nil).Twice()
	task1, err = s.queue.Pop(queueKey)
	s.NoError(err)
	s.NotNil(task1)
	s.Equal(task.Job.Signature(), task1.Job.Signature())

	count, err = s.queue.instance.LLen(context.Background(), queueKey).Result()
	s.NoError(err)
	s.Equal(int64(0), count)

	count, err = s.queue.instance.ZCount(context.Background(), s.queue.delayQueueKey(queueKey), "-inf", "+inf").Result()
	s.NoError(err)
	s.Equal(int64(0), count)
}

func (s *QueueTestSuite) TestMigrateDelayedJobs() {
	// Add a delayed job
	delay := time.Now().Add(-1 * time.Hour) // Past time
	payload := "test-payload"
	s.queue.instance.ZAdd(context.Background(), s.queue.delayQueueKey("test-queue"), redis.Z{
		Score:  float64(delay.Unix()),
		Member: payload,
	})

	err := s.queue.migrateDelayedJobs("test-queue")
	s.Nil(err)

	// Verify the job was moved to the main queue
	result, err := s.queue.instance.LPop(context.Background(), "test-queue").Result()
	s.Nil(err)
	s.Equal(payload, result)
}

var (
	mockArgs = []queue.Arg{
		{
			Type:  "string",
			Value: "value",
		},
		{
			Type:  "uint",
			Value: uint(1),
		},
		{
			Type:  "int",
			Value: 1,
		},
		{
			Type:  "bool",
			Value: true,
		},
		{
			Type:  "float32",
			Value: float32(1.1),
		},
		{
			Type:  "[]string",
			Value: []string{"a", "b", "c"},
		},
		{
			Type:  "[]int",
			Value: []int{1, 2, 3},
		},
		{
			Type:  "[]float32",
			Value: []float32{1.1, 2.2, 3.3},
		},
		{
			Type:  "[]bool",
			Value: []bool{true, false, true},
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
