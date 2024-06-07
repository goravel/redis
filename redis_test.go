package redis

import (
	"context"
	"testing"
	"time"

	configmock "github.com/goravel/framework/mocks/config"
	testingdocker "github.com/goravel/framework/support/docker"
	"github.com/goravel/framework/support/env"
	"github.com/spf13/cast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RedisTestSuite struct {
	suite.Suite
	mockConfig *configmock.Config
	redis      *Redis
}

func TestRedisTestSuite(t *testing.T) {
	if env.IsWindows() {
		t.Skip("Skipping tests of using docker")
	}

	redisDocker := testingdocker.NewRedis()
	assert.Nil(t, redisDocker.Build())

	mockConfig := &configmock.Config{}
	mockConfig.EXPECT().GetString("cache.stores.redis.connection", "default").Return("default").Once()
	mockConfig.EXPECT().GetString("database.redis.default.host").Return("localhost").Once()
	mockConfig.EXPECT().GetString("database.redis.default.port").Return(cast.ToString(redisDocker.Config().Port)).Once()
	mockConfig.EXPECT().GetString("database.redis.default.password").Return("").Once()
	mockConfig.EXPECT().GetInt("database.redis.default.database").Return(0).Once()
	mockConfig.EXPECT().Get("database.redis.default.tls").Return(nil).Once()
	mockConfig.EXPECT().GetString("cache.prefix").Return("goravel_cache").Once()
	store, err := NewRedis(context.Background(), mockConfig, "redis")
	require.NoError(t, err)
	mockConfig.AssertExpectations(t)

	suite.Run(t, &RedisTestSuite{
		redis: store,
	})

	assert.Nil(t, redisDocker.Stop())
}

func (s *RedisTestSuite) SetupTest() {
	s.mockConfig = &configmock.Config{}
}

func (s *RedisTestSuite) TestAdd() {
	s.Nil(s.redis.Put("name", "Goravel", 1*time.Second))
	s.False(s.redis.Add("name", "World", 1*time.Second))
	s.True(s.redis.Add("name1", "World", 1*time.Second))
	s.True(s.redis.Has("name1"))
	time.Sleep(2 * time.Second)
	s.False(s.redis.Has("name1"))
	s.True(s.redis.Flush())
}

func (s *RedisTestSuite) TestDecrement() {
	res, err := s.redis.Decrement("decrement")
	s.Equal(-1, res)
	s.Nil(err)

	s.Equal(-1, s.redis.GetInt("decrement"))

	res, err = s.redis.Decrement("decrement", 2)
	s.Equal(-3, res)
	s.Nil(err)

	res, err = s.redis.Decrement("decrement1", 2)
	s.Equal(-2, res)
	s.Nil(err)

	s.Equal(-2, s.redis.GetInt("decrement1"))

	s.True(s.redis.Add("decrement2", 4, 2*time.Second))
	res, err = s.redis.Decrement("decrement2")
	s.Equal(3, res)
	s.Nil(err)

	res, err = s.redis.Decrement("decrement2", 2)
	s.Equal(1, res)
	s.Nil(err)
}

func (s *RedisTestSuite) TestForever() {
	s.True(s.redis.Forever("name", "Goravel"))
	s.Equal("Goravel", s.redis.Get("name", "").(string))
	s.True(s.redis.Flush())
}

func (s *RedisTestSuite) TestForget() {
	val := s.redis.Forget("test-forget")
	s.True(val)

	err := s.redis.Put("test-forget", "goravel", 5*time.Second)
	s.Nil(err)
	s.True(s.redis.Forget("test-forget"))
}

func (s *RedisTestSuite) TestFlush() {
	s.Nil(s.redis.Put("test-flush", "goravel", 5*time.Second))
	s.Equal("goravel", s.redis.Get("test-flush", nil).(string))

	s.True(s.redis.Flush())
	s.False(s.redis.Has("test-flush"))
}

func (s *RedisTestSuite) TestGet() {
	s.Nil(s.redis.Put("name", "Goravel", 1*time.Second))
	s.Equal("Goravel", s.redis.Get("name", "").(string))
	s.Equal("World", s.redis.Get("name1", "World").(string))
	s.Equal("World1", s.redis.Get("name2", func() any {
		return "World1"
	}).(string))
	s.True(s.redis.Forget("name"))
	s.True(s.redis.Flush())
}

func (s *RedisTestSuite) TestGetBool() {
	s.Equal(true, s.redis.GetBool("test-get-bool", true))
	s.Nil(s.redis.Put("test-get-bool", true, 2*time.Second))
	s.Equal(true, s.redis.GetBool("test-get-bool", false))
}

func (s *RedisTestSuite) TestGetInt() {
	s.Equal(2, s.redis.GetInt("test-get-int", 2))
	s.Nil(s.redis.Put("test-get-int", 3, 2*time.Second))
	s.Equal(3, s.redis.GetInt("test-get-int", 2))
}

func (s *RedisTestSuite) TestGetString() {
	s.Equal("2", s.redis.GetString("test-get-string", "2"))
	s.Nil(s.redis.Put("test-get-string", "3", 2*time.Second))
	s.Equal("3", s.redis.GetString("test-get-string", "2"))
}

func (s *RedisTestSuite) TestHas() {
	s.False(s.redis.Has("test-has"))
	s.Nil(s.redis.Put("test-has", "goravel", 5*time.Second))
	s.True(s.redis.Has("test-has"))
}

func (s *RedisTestSuite) TestIncrement() {
	res, err := s.redis.Increment("Increment")
	s.Equal(1, res)
	s.Nil(err)

	s.Equal(1, s.redis.GetInt("Increment"))

	res, err = s.redis.Increment("Increment", 2)
	s.Equal(3, res)
	s.Nil(err)

	res, err = s.redis.Increment("Increment1", 2)
	s.Equal(2, res)
	s.Nil(err)

	s.Equal(2, s.redis.GetInt("Increment1"))

	s.True(s.redis.Add("Increment2", 1, 2*time.Second))
	res, err = s.redis.Increment("Increment2")
	s.Equal(2, res)
	s.Nil(err)

	res, err = s.redis.Increment("Increment2", 2)
	s.Equal(4, res)
	s.Nil(err)
}

func (s *RedisTestSuite) TestLock() {
	tests := []struct {
		name  string
		setup func()
	}{
		{
			name: "once got lock, lock can't be got again",
			setup: func() {
				lock := s.redis.Lock("lock")
				s.True(lock.Get())

				lock1 := s.redis.Lock("lock")
				s.False(lock1.Get())

				lock.Release()
			},
		},
		{
			name: "lock can be got again when had been released",
			setup: func() {
				lock := s.redis.Lock("lock")
				s.True(lock.Get())

				s.True(lock.Release())

				lock1 := s.redis.Lock("lock")
				s.True(lock1.Get())

				s.True(lock1.Release())
			},
		},
		{
			name: "lock cannot be released when had been got",
			setup: func() {
				lock := s.redis.Lock("lock")
				s.True(lock.Get())

				lock1 := s.redis.Lock("lock")
				s.False(lock1.Get())
				s.False(lock1.Release())

				s.True(lock.Release())
			},
		},
		{
			name: "lock can be force released",
			setup: func() {
				lock := s.redis.Lock("lock")
				s.True(lock.Get())

				lock1 := s.redis.Lock("lock")
				s.False(lock1.Get())
				s.False(lock1.Release())
				s.True(lock1.ForceRelease())

				s.True(lock.Release())
			},
		},
		{
			name: "lock can be got again when timeout",
			setup: func() {
				lock := s.redis.Lock("lock", 1*time.Second)
				s.True(lock.Get())

				time.Sleep(2 * time.Second)

				lock1 := s.redis.Lock("lock")
				s.True(lock1.Get())
				s.True(lock1.Release())
			},
		},
		{
			name: "lock can be got again when had been released by callback",
			setup: func() {
				lock := s.redis.Lock("lock")
				s.True(lock.Get(func() {
					s.True(true)
				}))

				lock1 := s.redis.Lock("lock")
				s.True(lock1.Get())
				s.True(lock1.Release())
			},
		},
		{
			name: "block wait out",
			setup: func() {
				lock := s.redis.Lock("lock")
				s.True(lock.Get())

				go func() {
					lock1 := s.redis.Lock("lock")
					s.NotNil(lock1.Block(1 * time.Second))
				}()

				time.Sleep(2 * time.Second)

				lock.Release()
			},
		},
		{
			name: "get lock by block when just timeout",
			setup: func() {
				lock := s.redis.Lock("lock")
				s.True(lock.Get())

				go func() {
					lock1 := s.redis.Lock("lock")
					s.True(lock1.Block(2 * time.Second))
					s.True(lock1.Release())
				}()

				time.Sleep(1 * time.Second)

				lock.Release()

				time.Sleep(2 * time.Second)
			},
		},
		{
			name: "get lock by block",
			setup: func() {
				lock := s.redis.Lock("lock")
				s.True(lock.Get())

				go func() {
					lock1 := s.redis.Lock("lock")
					s.True(lock1.Block(3 * time.Second))
					s.True(lock1.Release())
				}()

				time.Sleep(1 * time.Second)

				lock.Release()

				time.Sleep(3 * time.Second)
			},
		},
		{
			name: "get lock by block with callback",
			setup: func() {
				lock := s.redis.Lock("lock")
				s.True(lock.Get())

				go func() {
					lock1 := s.redis.Lock("lock")
					s.True(lock1.Block(2*time.Second, func() {
						s.True(true)
					}))
				}()

				time.Sleep(1 * time.Second)

				lock.Release()

				time.Sleep(2 * time.Second)
			},
		},
	}

	for _, test := range tests {
		s.Run(test.name, func() {
			test.setup()
		})
	}
}

func (s *RedisTestSuite) TestPull() {
	s.Nil(s.redis.Put("name", "Goravel", 1*time.Second))
	s.True(s.redis.Has("name"))
	s.Equal("Goravel", s.redis.Pull("name", "").(string))
	s.False(s.redis.Has("name"))
}

func (s *RedisTestSuite) TestPut() {
	s.Nil(s.redis.Put("name", "Goravel", 1*time.Second))
	s.True(s.redis.Has("name"))
	s.Equal("Goravel", s.redis.Get("name", "").(string))
	time.Sleep(2 * time.Second)
	s.False(s.redis.Has("name"))
}

func (s *RedisTestSuite) TestRemember() {
	s.Nil(s.redis.Put("name", "Goravel", 1*time.Second))
	value, err := s.redis.Remember("name", 1*time.Second, func() (any, error) {
		return "World", nil
	})
	s.Nil(err)
	s.Equal("Goravel", value)

	value, err = s.redis.Remember("name1", 1*time.Second, func() (any, error) {
		return "World1", nil
	})
	s.Nil(err)
	s.Equal("World1", value)
	time.Sleep(2 * time.Second)
	s.False(s.redis.Has("name1"))
	s.True(s.redis.Flush())
}

func (s *RedisTestSuite) TestRememberForever() {
	s.Nil(s.redis.Put("name", "Goravel", 1*time.Second))
	value, err := s.redis.RememberForever("name", func() (any, error) {
		return "World", nil
	})
	s.Nil(err)
	s.Equal("Goravel", value)

	value, err = s.redis.RememberForever("name1", func() (any, error) {
		return "World1", nil
	})
	s.Nil(err)
	s.Equal("World1", value)
	s.True(s.redis.Flush())
}
