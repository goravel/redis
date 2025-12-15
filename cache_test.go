package redis

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	mocksconfig "github.com/goravel/framework/mocks/config"
	"github.com/goravel/framework/support/env"
	"github.com/spf13/cast"
	"github.com/stretchr/testify/suite"
)

type CacheTestSuite struct {
	suite.Suite
	docker *Docker
	cache  *Cache
}

func TestCacheTestSuite(t *testing.T) {
	if env.IsWindows() {
		t.Skip("Skipping tests of using docker")
	}

	suite.Run(t, &CacheTestSuite{})
}

func (s *CacheTestSuite) SetupSuite() {
	mockConfig := mocksconfig.NewConfig(s.T())
	docker := initDocker(mockConfig)

	mockGetClient(mockConfig, docker)

	mockConfig.EXPECT().GetString(fmt.Sprintf("cache.stores.%s.connection", testStore), "default").Return(testConnection).Once()
	mockConfig.EXPECT().GetString("cache.prefix").Return("goravel_cache").Once()

	store, err := NewCache(context.Background(), mockConfig, testStore)
	s.Require().NoError(err)

	s.cache = store
	s.docker = docker
}

func (s *CacheTestSuite) TearDownSuite() {
	s.NoError(s.docker.Shutdown())
}

func (s *CacheTestSuite) SetupTest() {}

func (s *CacheTestSuite) TestAdd() {
	s.Nil(s.cache.Put("name", "Goravel", 1*time.Second))
	s.False(s.cache.Add("name", "World", 1*time.Second))
	s.True(s.cache.Add("name1", "World", 1*time.Second))
	s.True(s.cache.Has("name1"))
	time.Sleep(2 * time.Second)
	s.False(s.cache.Has("name1"))
	s.True(s.cache.Flush())
}

func (s *CacheTestSuite) TestDecrement() {
	res, err := s.cache.Decrement("decrement")
	s.Equal(int64(-1), res)
	s.Nil(err)

	s.Equal(int64(-1), s.cache.GetInt64("decrement"))

	res, err = s.cache.Decrement("decrement", 2)
	s.Equal(int64(-3), res)
	s.Nil(err)

	res, err = s.cache.Decrement("decrement1", 2)
	s.Equal(int64(-2), res)
	s.Nil(err)

	s.Equal(int64(-2), s.cache.GetInt64("decrement1"))

	decrement2 := int64(4)
	s.True(s.cache.Add("decrement2", &decrement2, 2*time.Second))
	res, err = s.cache.Decrement("decrement2")
	s.Equal(int64(3), res)
	s.Nil(err)

	res, err = s.cache.Decrement("decrement2", 2)
	s.Equal(int64(1), res)
	s.Nil(err)
}

func (s *CacheTestSuite) TestDecrementWithConcurrent() {
	res, err := s.cache.Decrement("decrement_concurrent")
	s.Equal(int64(-1), res)
	s.Nil(err)

	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			_, err = s.cache.Decrement("decrement_concurrent", 1)
			s.Nil(err)
			wg.Done()
		}()
	}

	wg.Wait()

	res = s.cache.GetInt64("decrement_concurrent")
	s.Equal(int64(-1001), res)
	s.Nil(err)
}

func (s *CacheTestSuite) TestForever() {
	s.True(s.cache.Forever("name", "Goravel"))
	s.Equal("Goravel", s.cache.Get("name", "").(string))
	s.True(s.cache.Flush())
}

func (s *CacheTestSuite) TestForget() {
	val := s.cache.Forget("test-forget")
	s.True(val)

	err := s.cache.Put("test-forget", "goravel", 5*time.Second)
	s.Nil(err)
	s.True(s.cache.Forget("test-forget"))
}

func (s *CacheTestSuite) TestFlush() {
	s.Nil(s.cache.Put("test-flush", "goravel", 5*time.Second))
	s.Equal("goravel", s.cache.Get("test-flush", nil).(string))

	s.True(s.cache.Flush())
	s.False(s.cache.Has("test-flush"))
}

func (s *CacheTestSuite) TestGet() {
	s.Nil(s.cache.Put("name", "Goravel", 1*time.Second))
	s.Equal("Goravel", s.cache.Get("name", "").(string))
	s.Equal("World", s.cache.Get("name1", "World").(string))
	s.Equal("World1", s.cache.Get("name2", func() any {
		return "World1"
	}).(string))
	s.True(s.cache.Forget("name"))
	s.True(s.cache.Flush())
}

func (s *CacheTestSuite) TestGetBool() {
	s.Equal(true, s.cache.GetBool("test-get-bool", true))
	s.Nil(s.cache.Put("test-get-bool", true, 2*time.Second))
	s.Equal(true, s.cache.GetBool("test-get-bool", false))
}

func (s *CacheTestSuite) TestGetInt() {
	s.Equal(2, s.cache.GetInt("test-get-int", 2))
	s.Nil(s.cache.Put("test-get-int", 3, 2*time.Second))
	s.Equal(3, s.cache.GetInt("test-get-int", 2))
}

func (s *CacheTestSuite) TestGetString() {
	s.Equal("2", s.cache.GetString("test-get-string", "2"))
	s.Nil(s.cache.Put("test-get-string", "3", 2*time.Second))
	s.Equal("3", s.cache.GetString("test-get-string", "2"))
}

func (s *CacheTestSuite) TestHas() {
	s.False(s.cache.Has("test-has"))
	s.Nil(s.cache.Put("test-has", "goravel", 5*time.Second))
	s.True(s.cache.Has("test-has"))
}

func (s *CacheTestSuite) TestIncrement() {
	res, err := s.cache.Increment("Increment")
	s.Equal(int64(1), res)
	s.Nil(err)

	s.Equal(int64(1), s.cache.GetInt64("Increment"))

	res, err = s.cache.Increment("Increment", 2)
	s.Equal(int64(3), res)
	s.Nil(err)

	res, err = s.cache.Increment("Increment1", 2)
	s.Equal(int64(2), res)
	s.Nil(err)

	s.Equal(int64(2), s.cache.GetInt64("Increment1"))

	increment2 := int64(1)
	s.True(s.cache.Add("Increment2", &increment2, 2*time.Second))
	res, err = s.cache.Increment("Increment2")
	s.Equal(int64(2), res)
	s.Nil(err)

	res, err = s.cache.Increment("Increment2", 2)
	s.Equal(int64(4), res)
	s.Nil(err)
}

func (s *CacheTestSuite) TestIncrementWithConcurrent() {
	res, err := s.cache.Increment("decrement_concurrent")
	s.Equal(int64(1), res)
	s.Nil(err)

	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			_, err = s.cache.Increment("decrement_concurrent", 1)
			s.Nil(err)
			wg.Done()
		}()
	}

	wg.Wait()

	res = s.cache.GetInt64("decrement_concurrent")
	s.Equal(int64(1001), res)
	s.Nil(err)
}

func (s *CacheTestSuite) TestLock() {
	tests := []struct {
		name  string
		setup func()
	}{
		{
			name: "once got lock, lock can't be got again",
			setup: func() {
				lock := s.cache.Lock("lock")
				s.True(lock.Get())

				lock1 := s.cache.Lock("lock")
				s.False(lock1.Get())

				lock.Release()
			},
		},
		{
			name: "lock can be got again when had been released",
			setup: func() {
				lock := s.cache.Lock("lock")
				s.True(lock.Get())

				s.True(lock.Release())

				lock1 := s.cache.Lock("lock")
				s.True(lock1.Get())

				s.True(lock1.Release())
			},
		},
		{
			name: "lock cannot be released when had been got",
			setup: func() {
				lock := s.cache.Lock("lock")
				s.True(lock.Get())

				lock1 := s.cache.Lock("lock")
				s.False(lock1.Get())
				s.False(lock1.Release())

				s.True(lock.Release())
			},
		},
		{
			name: "lock can be force released",
			setup: func() {
				lock := s.cache.Lock("lock")
				s.True(lock.Get())

				lock1 := s.cache.Lock("lock")
				s.False(lock1.Get())
				s.False(lock1.Release())
				s.True(lock1.ForceRelease())

				s.True(lock.Release())
			},
		},
		{
			name: "lock can be got again when timeout",
			setup: func() {
				lock := s.cache.Lock("lock", 1*time.Second)
				s.True(lock.Get())

				time.Sleep(2 * time.Second)

				lock1 := s.cache.Lock("lock")
				s.True(lock1.Get())
				s.True(lock1.Release())
			},
		},
		{
			name: "lock can be got again when had been released by callback",
			setup: func() {
				lock := s.cache.Lock("lock")
				s.True(lock.Get(func() {
					s.True(true)
				}))

				lock1 := s.cache.Lock("lock")
				s.True(lock1.Get())
				s.True(lock1.Release())
			},
		},
		{
			name: "block wait out",
			setup: func() {
				lock := s.cache.Lock("lock")
				s.True(lock.Get())

				go func() {
					lock1 := s.cache.Lock("lock")
					s.NotNil(lock1.Block(1 * time.Second))
				}()

				time.Sleep(2 * time.Second)

				lock.Release()
			},
		},
		{
			name: "get lock by block when just timeout",
			setup: func() {
				lock := s.cache.Lock("lock")
				s.True(lock.Get())

				go func() {
					lock1 := s.cache.Lock("lock")
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
				lock := s.cache.Lock("lock")
				s.True(lock.Get())

				go func() {
					lock1 := s.cache.Lock("lock")
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
				lock := s.cache.Lock("lock")
				s.True(lock.Get())

				go func() {
					lock1 := s.cache.Lock("lock")
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

func (s *CacheTestSuite) TestPull() {
	s.Nil(s.cache.Put("name", "Goravel", 1*time.Second))
	s.True(s.cache.Has("name"))
	s.Equal("Goravel", s.cache.Pull("name", "").(string))
	s.False(s.cache.Has("name"))
}

func (s *CacheTestSuite) TestPut() {
	s.Nil(s.cache.Put("name", "Goravel", 1*time.Second))
	s.True(s.cache.Has("name"))
	s.Equal("Goravel", s.cache.Get("name", "").(string))
	time.Sleep(2 * time.Second)
	s.False(s.cache.Has("name"))
}

func (s *CacheTestSuite) TestRemember() {
	s.Nil(s.cache.Put("name", "Goravel", 1*time.Second))
	value, err := s.cache.Remember("name", 1*time.Second, func() (any, error) {
		return "World", nil
	})
	s.Nil(err)
	s.Equal("Goravel", value)

	value, err = s.cache.Remember("name1", 1*time.Second, func() (any, error) {
		return "World1", nil
	})
	s.Nil(err)
	s.Equal("World1", value)
	time.Sleep(2 * time.Second)
	s.False(s.cache.Has("name1"))
	s.True(s.cache.Flush())
}

func (s *CacheTestSuite) TestRememberForever() {
	s.Nil(s.cache.Put("name", "Goravel", 1*time.Second))
	value, err := s.cache.RememberForever("name", func() (any, error) {
		return "World", nil
	})
	s.Nil(err)
	s.Equal("Goravel", value)

	value, err = s.cache.RememberForever("name1", func() (any, error) {
		return "World1", nil
	})
	s.Nil(err)
	s.Equal("World1", value)
	s.True(s.cache.Flush())
}

func mockGetClient(mockConfig *mocksconfig.Config, docker *Docker) {
	mockConfig.EXPECT().GetString(fmt.Sprintf("database.redis.%s.host", testConnection)).Return(docker.Config().Host).Once()
	mockConfig.EXPECT().GetString(fmt.Sprintf("database.redis.%s.port", testConnection), "6379").Return(cast.ToString(docker.Config().Port)).Once()
	mockConfig.EXPECT().GetString(fmt.Sprintf("database.redis.%s.username", testConnection)).Return(docker.Config().Username).Once()
	mockConfig.EXPECT().GetString(fmt.Sprintf("database.redis.%s.password", testConnection)).Return(docker.Config().Password).Once()
	mockConfig.EXPECT().GetInt(fmt.Sprintf("database.redis.%s.database", testConnection), 0).Return(cast.ToInt(docker.Config().Database)).Once()
	mockConfig.EXPECT().GetBool(fmt.Sprintf("database.redis.%s.cluster", testConnection), false).Return(false).Once()
	mockConfig.EXPECT().Get(fmt.Sprintf("database.redis.%s.tls", testConnection)).Return(nil).Once()
}
