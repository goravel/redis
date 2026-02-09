package redis

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	contractsqueue "github.com/goravel/framework/contracts/queue"
	"github.com/goravel/framework/errors"
	"github.com/goravel/framework/foundation/json"
	mocksconfig "github.com/goravel/framework/mocks/config"
	mocksqueue "github.com/goravel/framework/mocks/queue"
	"github.com/goravel/framework/support/carbon"
	"github.com/goravel/framework/support/env"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type QueueTestSuite struct {
	suite.Suite
	mockQueue     *mocksqueue.Queue
	mockJobStorer *mocksqueue.JobStorer
	ctx           context.Context
	queue         *Queue
	docker        *Docker
}

func TestQueueTestSuite(t *testing.T) {
	if env.IsWindows() {
		t.Skip("Skipping tests of using docker")
	}

	suite.Run(t, &QueueTestSuite{})
}

func (s *QueueTestSuite) SetupSuite() {
	mockConfig := mocksconfig.NewConfig(s.T())
	docker := initDocker(mockConfig)

	mockGetClient(mockConfig, docker)

	mockConfig.EXPECT().GetString("app.name", "goravel").Return("test").Once()
	mockConfig.EXPECT().GetString(fmt.Sprintf("queue.connections.%s.connection", testConnection), "default").Return(testConnection).Once()

	mockQueue := mocksqueue.NewQueue(s.T())
	s.mockJobStorer = mocksqueue.NewJobStorer(s.T())
	mockQueue.EXPECT().JobStorer().Return(s.mockJobStorer).Once()

	s.mockQueue = mockQueue
	s.ctx = context.Background()

	queue, err := NewQueue(s.ctx, mockConfig, mockQueue, json.New(), testConnection)
	s.Nil(err)

	s.docker = docker
	s.queue = queue
}

func (s *QueueTestSuite) TearDownSuite() {
	s.NoError(s.docker.Shutdown())
}

func (s *QueueTestSuite) SetupTest() {
	clients.Clear()
}

func (s *QueueTestSuite) Test_Driver() {
	s.Equal("custom", s.queue.Driver())
}

func (s *QueueTestSuite) Test_Push() {
	carbon.SetTestNow(carbon.Parse("2025-05-28 18:50:57"))
	defer carbon.ClearTestNow()

	s.Run("no delay", func() {
		queue := "no-delay"
		task := contractsqueue.Task{
			UUID: "865111de-ff50-4652-9733-72fea655f836",
			ChainJob: contractsqueue.ChainJob{
				Job:  &MockJob{},
				Args: testArgs,
			},
			Chain: []contractsqueue.ChainJob{
				{
					Job:   &MockJob{},
					Args:  testArgs,
					Delay: carbon.Now().AddHour().StdTime(),
				},
			},
		}

		s.NoError(s.queue.Push(task, queue))

		count, err := s.queue.client.LLen(s.ctx, s.queue.queueKey.Queue(queue)).Result()
		s.NoError(err)
		s.Equal(int64(1), count)

		result, err := s.queue.client.LPop(s.ctx, s.queue.queueKey.Queue(queue)).Result()
		s.NoError(err)
		s.Equal(`{"playload":"{\"delay\":null,\"signature\":\"mock\",\"args\":[{\"value\":true,\"type\":\"bool\"},{\"value\":1,\"type\":\"int\"},{\"value\":1,\"type\":\"int8\"},{\"value\":1,\"type\":\"int16\"},{\"value\":1,\"type\":\"int32\"},{\"value\":1,\"type\":\"int64\"},{\"value\":1,\"type\":\"uint\"},{\"value\":1,\"type\":\"uint8\"},{\"value\":1,\"type\":\"uint16\"},{\"value\":1,\"type\":\"uint32\"},{\"value\":1,\"type\":\"uint64\"},{\"value\":1.1,\"type\":\"float32\"},{\"value\":1.2,\"type\":\"float64\"},{\"value\":\"test\",\"type\":\"string\"},{\"value\":[true,false],\"type\":\"[]bool\"},{\"value\":[1,2,3],\"type\":\"[]int\"},{\"value\":[1,2,3],\"type\":\"[]int8\"},{\"value\":[1,2,3],\"type\":\"[]int16\"},{\"value\":[1,2,3],\"type\":\"[]int32\"},{\"value\":[1,2,3],\"type\":\"[]int64\"},{\"value\":[1,2,3],\"type\":\"[]uint\"},{\"value\":[1,2,3],\"type\":\"[]uint8\"},{\"value\":[1,2,3],\"type\":\"[]uint16\"},{\"value\":[1,2,3],\"type\":\"[]uint32\"},{\"value\":[1,2,3],\"type\":\"[]uint64\"},{\"value\":[1.1,1.2,1.3],\"type\":\"[]float32\"},{\"value\":[1.1,1.2,1.3],\"type\":\"[]float64\"},{\"value\":[\"test\",\"test2\",\"test3\"],\"type\":\"[]string\"}],\"uuid\":\"865111de-ff50-4652-9733-72fea655f836\",\"chain\":[{\"delay\":\"2025-05-28T19:50:57Z\",\"signature\":\"mock\",\"args\":[{\"value\":true,\"type\":\"bool\"},{\"value\":1,\"type\":\"int\"},{\"value\":1,\"type\":\"int8\"},{\"value\":1,\"type\":\"int16\"},{\"value\":1,\"type\":\"int32\"},{\"value\":1,\"type\":\"int64\"},{\"value\":1,\"type\":\"uint\"},{\"value\":1,\"type\":\"uint8\"},{\"value\":1,\"type\":\"uint16\"},{\"value\":1,\"type\":\"uint32\"},{\"value\":1,\"type\":\"uint64\"},{\"value\":1.1,\"type\":\"float32\"},{\"value\":1.2,\"type\":\"float64\"},{\"value\":\"test\",\"type\":\"string\"},{\"value\":[true,false],\"type\":\"[]bool\"},{\"value\":[1,2,3],\"type\":\"[]int\"},{\"value\":[1,2,3],\"type\":\"[]int8\"},{\"value\":[1,2,3],\"type\":\"[]int16\"},{\"value\":[1,2,3],\"type\":\"[]int32\"},{\"value\":[1,2,3],\"type\":\"[]int64\"},{\"value\":[1,2,3],\"type\":\"[]uint\"},{\"value\":[1,2,3],\"type\":\"[]uint8\"},{\"value\":[1,2,3],\"type\":\"[]uint16\"},{\"value\":[1,2,3],\"type\":\"[]uint32\"},{\"value\":[1,2,3],\"type\":\"[]uint64\"},{\"value\":[1.1,1.2,1.3],\"type\":\"[]float32\"},{\"value\":[1.1,1.2,1.3],\"type\":\"[]float64\"},{\"value\":[\"test\",\"test2\",\"test3\"],\"type\":\"[]string\"}]}]}","attempts":0,"reserved_at":null}`, result)

		count, err = s.queue.client.LLen(s.ctx, s.queue.queueKey.Delayed(queue)).Result()
		s.NoError(err)
		s.Equal(int64(0), count)
	})

	s.Run("delay", func() {
		queue := "delay"
		task := contractsqueue.Task{
			UUID: "865111de-ff50-4652-9733-72fea655f836",
			ChainJob: contractsqueue.ChainJob{
				Job:   &MockJob{},
				Args:  testArgs,
				Delay: time.Now().Add(2 * time.Second),
			},
			Chain: []contractsqueue.ChainJob{
				{
					Job:   &MockJob{},
					Args:  testArgs,
					Delay: carbon.Now().AddSecond().StdTime(),
				},
			},
		}

		s.NoError(s.queue.Push(task, queue))

		count, err := s.queue.client.LLen(context.Background(), s.queue.queueKey.Queue(queue)).Result()
		s.NoError(err)
		s.Equal(int64(0), count)

		count, err = s.queue.client.ZCount(context.Background(), s.queue.queueKey.Delayed(queue), "-inf", "+inf").Result()
		s.NoError(err)
		s.Equal(int64(1), count)

		time.Sleep(2 * time.Second)

		jobs, err := s.queue.client.ZRangeByScoreWithScores(s.ctx, s.queue.queueKey.Delayed(queue), &redis.ZRangeBy{
			Min:    "-inf",
			Max:    strconv.FormatFloat(float64(time.Now().Unix()), 'f', -1, 64),
			Offset: 0,
			Count:  -1,
		}).Result()
		s.NoError(err)
		s.Equal(1, len(jobs))
		s.Equal(`{"playload":"{\"delay\":null,\"signature\":\"mock\",\"args\":[{\"value\":true,\"type\":\"bool\"},{\"value\":1,\"type\":\"int\"},{\"value\":1,\"type\":\"int8\"},{\"value\":1,\"type\":\"int16\"},{\"value\":1,\"type\":\"int32\"},{\"value\":1,\"type\":\"int64\"},{\"value\":1,\"type\":\"uint\"},{\"value\":1,\"type\":\"uint8\"},{\"value\":1,\"type\":\"uint16\"},{\"value\":1,\"type\":\"uint32\"},{\"value\":1,\"type\":\"uint64\"},{\"value\":1.1,\"type\":\"float32\"},{\"value\":1.2,\"type\":\"float64\"},{\"value\":\"test\",\"type\":\"string\"},{\"value\":[true,false],\"type\":\"[]bool\"},{\"value\":[1,2,3],\"type\":\"[]int\"},{\"value\":[1,2,3],\"type\":\"[]int8\"},{\"value\":[1,2,3],\"type\":\"[]int16\"},{\"value\":[1,2,3],\"type\":\"[]int32\"},{\"value\":[1,2,3],\"type\":\"[]int64\"},{\"value\":[1,2,3],\"type\":\"[]uint\"},{\"value\":[1,2,3],\"type\":\"[]uint8\"},{\"value\":[1,2,3],\"type\":\"[]uint16\"},{\"value\":[1,2,3],\"type\":\"[]uint32\"},{\"value\":[1,2,3],\"type\":\"[]uint64\"},{\"value\":[1.1,1.2,1.3],\"type\":\"[]float32\"},{\"value\":[1.1,1.2,1.3],\"type\":\"[]float64\"},{\"value\":[\"test\",\"test2\",\"test3\"],\"type\":\"[]string\"}],\"uuid\":\"865111de-ff50-4652-9733-72fea655f836\",\"chain\":[{\"delay\":\"2025-05-28T18:50:58Z\",\"signature\":\"mock\",\"args\":[{\"value\":true,\"type\":\"bool\"},{\"value\":1,\"type\":\"int\"},{\"value\":1,\"type\":\"int8\"},{\"value\":1,\"type\":\"int16\"},{\"value\":1,\"type\":\"int32\"},{\"value\":1,\"type\":\"int64\"},{\"value\":1,\"type\":\"uint\"},{\"value\":1,\"type\":\"uint8\"},{\"value\":1,\"type\":\"uint16\"},{\"value\":1,\"type\":\"uint32\"},{\"value\":1,\"type\":\"uint64\"},{\"value\":1.1,\"type\":\"float32\"},{\"value\":1.2,\"type\":\"float64\"},{\"value\":\"test\",\"type\":\"string\"},{\"value\":[true,false],\"type\":\"[]bool\"},{\"value\":[1,2,3],\"type\":\"[]int\"},{\"value\":[1,2,3],\"type\":\"[]int8\"},{\"value\":[1,2,3],\"type\":\"[]int16\"},{\"value\":[1,2,3],\"type\":\"[]int32\"},{\"value\":[1,2,3],\"type\":\"[]int64\"},{\"value\":[1,2,3],\"type\":\"[]uint\"},{\"value\":[1,2,3],\"type\":\"[]uint8\"},{\"value\":[1,2,3],\"type\":\"[]uint16\"},{\"value\":[1,2,3],\"type\":\"[]uint32\"},{\"value\":[1,2,3],\"type\":\"[]uint64\"},{\"value\":[1.1,1.2,1.3],\"type\":\"[]float32\"},{\"value\":[1.1,1.2,1.3],\"type\":\"[]float64\"},{\"value\":[\"test\",\"test2\",\"test3\"],\"type\":\"[]string\"}]}]}","attempts":0,"reserved_at":null}`, jobs[0].Member)
	})
}

func (s *QueueTestSuite) TestPop() {
	s.Run("no job", func() {
		queue := "no-job"
		task, err := s.queue.Pop(queue)

		s.Equal(errors.QueueDriverNoJobFound.Args(s.queue.queueKey.Queue(queue)), err)
		s.Nil(task)
	})

	s.Run("success", func() {
		queue := "pop"
		queueKey := s.queue.queueKey.Queue(queue)
		task := contractsqueue.Task{
			UUID: "865111de-ff50-4652-9733-72fea655f836",
			ChainJob: contractsqueue.ChainJob{
				Job:  &MockJob{},
				Args: testArgs,
			},
			Chain: []contractsqueue.ChainJob{
				{
					Job:   &MockJob{},
					Args:  testArgs,
					Delay: carbon.Now().AddSecond().StdTime(),
				},
			},
		}

		s.NoError(s.queue.Push(task, queue))

		s.mockJobStorer.EXPECT().Get(task.Job.Signature()).Return(&MockJob{}, nil).Twice()

		task1, err := s.queue.Pop(queue)

		s.NoError(err)
		s.Equal(task.Job.Signature(), task1.Task().Job.Signature())
		s.Equal(len(task.Args), len(task1.Task().Args))
		for i, arg := range task1.Task().Args {
			s.Equal(task.Args[i].Type, arg.Type)
		}
		s.Equal(task.Delay, task1.Task().Delay)
		s.Len(task1.Task().Chain, 1)
		for i, chained := range task1.Task().Chain {
			s.Equal(task.Chain[i].Job.Signature(), chained.Job.Signature())
			s.Equal(len(task.Chain[i].Args), len(chained.Args))
			for j, arg := range chained.Args {
				s.Equal(task.Chain[i].Args[j].Type, arg.Type)
			}
			s.Equal(task.Chain[i].Delay.Format(time.RFC3339), chained.Delay.Format(time.RFC3339))
		}

		count, err := s.queue.client.LLen(context.Background(), queueKey).Result()
		s.NoError(err)
		s.Equal(int64(0), count)
	})

	s.Run("concurrent pop with delayed job", func() {
		queue := "concurrent-delay"
		task := contractsqueue.Task{
			UUID: "concurrent-test-uuid",
			ChainJob: contractsqueue.ChainJob{
				Job:   &MockJob{},
				Args:  testArgs,
				Delay: time.Now().Add(1 * time.Second),
			},
		}

		// Push a delayed job
		s.NoError(s.queue.Push(task, queue))

		// Wait for the delay to pass
		time.Sleep(1100 * time.Millisecond)

		// Setup expectations for exactly one successful job retrieval
		s.mockJobStorer.EXPECT().Get(task.Job.Signature()).Return(&MockJob{}, nil).Once()

		// Simulate concurrent Pop calls
		concurrency := 5
		results := make(chan contractsqueue.ReservedJob, concurrency)
		errs := make(chan error, concurrency)

		for i := 0; i < concurrency; i++ {
			go func() {
				job, err := s.queue.Pop(queue)
				if err != nil {
					errs <- err
				} else {
					results <- job
				}
			}()
		}

		// Collect results
		var successCount int
		var noJobCount int
		timeout := time.After(2 * time.Second)

		for i := 0; i < concurrency; i++ {
			select {
			case job := <-results:
				successCount++
				s.NotNil(job)
				s.Equal(task.Job.Signature(), job.Task().Job.Signature())
			case err := <-errs:
				if errors.Is(err, errors.QueueDriverNoJobFound) {
					noJobCount++
				} else {
					s.Fail("unexpected error: %v", err)
				}
			case <-timeout:
				s.Fail("timeout waiting for Pop results")
			}
		}

		// Only one goroutine should have successfully retrieved the job
		s.Equal(1, successCount, "expected exactly one successful Pop")
		s.Equal(4, noJobCount, "expected four 'no job found' errors")

		// Verify the queue is empty
		count, err := s.queue.client.LLen(context.Background(), s.queue.queueKey.Queue(queue)).Result()
		s.NoError(err)
		s.Equal(int64(0), count)

		// Verify the delayed queue is empty
		delayedCount, err := s.queue.client.ZCount(context.Background(), s.queue.queueKey.Delayed(queue), "-inf", "+inf").Result()
		s.NoError(err)
		s.Equal(int64(0), delayedCount)
	})
}

func (s *QueueTestSuite) Test_Later() {
	queue := "later"
	queueKey := s.queue.queueKey.Queue(queue)
	task := contractsqueue.Task{
		UUID: "865111de-ff50-4652-9733-72fea655f836",
		ChainJob: contractsqueue.ChainJob{
			Job:   &MockJob{},
			Args:  testArgs,
			Delay: time.Now().Add(1 * time.Second),
		},
		Chain: []contractsqueue.ChainJob{
			{
				Job:   &MockJob{},
				Args:  testArgs,
				Delay: time.Now().Add(1 * time.Hour),
			},
		},
	}

	s.NoError(s.queue.Later(task.Delay, task, queue))

	count, err := s.queue.client.LLen(context.Background(), queueKey).Result()
	s.NoError(err)
	s.Equal(int64(0), count)

	count, err = s.queue.client.ZCount(context.Background(), s.queue.queueKey.Delayed(queue), "-inf", "+inf").Result()
	s.NoError(err)
	s.Equal(int64(1), count)

	task1, err := s.queue.Pop(queue)

	s.Equal(errors.QueueDriverNoJobFound.Args(queueKey), err)
	s.Nil(task1)

	time.Sleep(1 * time.Second)

	s.mockJobStorer.EXPECT().Get(task.Job.Signature()).Return(&MockJob{}, nil).Twice()

	task1, err = s.queue.Pop(queue)
	s.NoError(err)
	s.NotNil(task1)
	s.Equal(task.Job.Signature(), task1.Task().Job.Signature())

	count, err = s.queue.client.LLen(context.Background(), queueKey).Result()
	s.NoError(err)
	s.Equal(int64(0), count)

	count, err = s.queue.client.ZCount(context.Background(), s.queue.queueKey.Delayed(queue), "-inf", "+inf").Result()
	s.NoError(err)
	s.Equal(int64(0), count)
}

func (s *QueueTestSuite) Test_migrateDelayedJobs() {
	s.Run("migrate ready job", func() {
		queue := "test-queue-migrate"
		delay := time.Now().Add(-1 * time.Hour) // Past time
		payload := "test-payload"
		s.queue.client.ZAdd(context.Background(), s.queue.queueKey.Delayed(queue), redis.Z{
			Score:  float64(delay.Unix()),
			Member: payload,
		})

		err := s.queue.migrateDelayedJobs(queue)
		s.Nil(err)

		// Verify the job was moved to the main queue
		result, err := s.queue.client.LPop(context.Background(), s.queue.queueKey.Queue(queue)).Result()
		s.Nil(err)
		s.Equal(payload, result)

		// Verify delayed queue is empty
		count, err := s.queue.client.ZCount(context.Background(), s.queue.queueKey.Delayed(queue), "-inf", "+inf").Result()
		s.NoError(err)
		s.Equal(int64(0), count)
	})

	s.Run("do not migrate future job", func() {
		queue := "test-queue-future"
		delay := time.Now().Add(1 * time.Hour) // Future time
		payload := "future-payload"
		s.queue.client.ZAdd(context.Background(), s.queue.queueKey.Delayed(queue), redis.Z{
			Score:  float64(delay.Unix()),
			Member: payload,
		})

		err := s.queue.migrateDelayedJobs(queue)
		s.Nil(err)

		// Verify the job is still in the delayed queue
		count, err := s.queue.client.ZCount(context.Background(), s.queue.queueKey.Delayed(queue), "-inf", "+inf").Result()
		s.NoError(err)
		s.Equal(int64(1), count)

		// Verify main queue is empty
		mainCount, err := s.queue.client.LLen(context.Background(), s.queue.queueKey.Queue(queue)).Result()
		s.NoError(err)
		s.Equal(int64(0), mainCount)
	})

	s.Run("migrate multiple jobs in order", func() {
		queue := "test-queue-multiple"
		payload1 := "payload-1"
		payload2 := "payload-2"
		payload3 := "payload-3"

		// Add jobs with different delays (all in the past)
		s.queue.client.ZAdd(context.Background(), s.queue.queueKey.Delayed(queue), redis.Z{
			Score:  float64(time.Now().Add(-3 * time.Hour).Unix()),
			Member: payload1,
		})
		s.queue.client.ZAdd(context.Background(), s.queue.queueKey.Delayed(queue), redis.Z{
			Score:  float64(time.Now().Add(-2 * time.Hour).Unix()),
			Member: payload2,
		})
		s.queue.client.ZAdd(context.Background(), s.queue.queueKey.Delayed(queue), redis.Z{
			Score:  float64(time.Now().Add(-1 * time.Hour).Unix()),
			Member: payload3,
		})

		err := s.queue.migrateDelayedJobs(queue)
		s.Nil(err)

		// Verify all jobs were migrated
		count, err := s.queue.client.LLen(context.Background(), s.queue.queueKey.Queue(queue)).Result()
		s.NoError(err)
		s.Equal(int64(3), count)

		// Verify delayed queue is empty
		delayedCount, err := s.queue.client.ZCount(context.Background(), s.queue.queueKey.Delayed(queue), "-inf", "+inf").Result()
		s.NoError(err)
		s.Equal(int64(0), delayedCount)

		// Verify order: oldest first
		result1, _ := s.queue.client.LPop(context.Background(), s.queue.queueKey.Queue(queue)).Result()
		result2, _ := s.queue.client.LPop(context.Background(), s.queue.queueKey.Queue(queue)).Result()
		result3, _ := s.queue.client.LPop(context.Background(), s.queue.queueKey.Queue(queue)).Result()
		s.Equal(payload1, result1)
		s.Equal(payload2, result2)
		s.Equal(payload3, result3)
	})
}

func TestQueueKey(t *testing.T) {
	queueKey := NewQueueKey("test-app", "test-connection")

	t.Run("Queue method", func(t *testing.T) {
		expected := "test-app_queues:test-connection_test-queue"
		actual := queueKey.Queue("test-queue")

		assert.Equal(t, expected, actual)
	})

	t.Run("Delayed method", func(t *testing.T) {
		expected := "test-app_queues:test-connection_test-queue:delayed"
		actual := queueKey.Delayed("test-queue")

		assert.Equal(t, expected, actual)
	})

	t.Run("Reserved method", func(t *testing.T) {
		expected := "test-app_queues:test-connection_test-queue:reserved"
		actual := queueKey.Reserved("test-queue")

		assert.Equal(t, expected, actual)
	})
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
