package redis

import (
	"log"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/suite"

	configmocks "github.com/goravel/framework/mocks/config"
)

type CacheTestSuite struct {
	suite.Suite
	mockConfig  *configmocks.Config
	redis       *Cache
	redisDocker *dockertest.Resource
}

func TestCacheTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping tests of using docker")
	}

	redisPool, redisDocker, redisStore, err := getCacheDocker()
	if err != nil {
		log.Fatalf("Get redis store error: %s", err)
	}

	suite.Run(t, &CacheTestSuite{
		redisDocker: redisDocker,
		redis:       redisStore,
	})

	if err := redisPool.Purge(redisDocker); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}
}

func (s *CacheTestSuite) SetupTest() {
	s.mockConfig = &configmocks.Config{}
}

func (s *CacheTestSuite) TestAdd() {
	s.Nil(s.redis.Put("name", "Goravel", 1*time.Second))
	s.False(s.redis.Add("name", "World", 1*time.Second))
	s.True(s.redis.Add("name1", "World", 1*time.Second))
	s.True(s.redis.Has("name1"))
	time.Sleep(2 * time.Second)
	s.False(s.redis.Has("name1"))
	s.True(s.redis.Flush())
}

func (s *CacheTestSuite) TestDecrement() {
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

func (s *CacheTestSuite) TestForever() {
	s.True(s.redis.Forever("name", "Goravel"))
	s.Equal("Goravel", s.redis.Get("name", "").(string))
	s.True(s.redis.Flush())
}

func (s *CacheTestSuite) TestForget() {
	val := s.redis.Forget("test-forget")
	s.True(val)

	err := s.redis.Put("test-forget", "goravel", 5*time.Second)
	s.Nil(err)
	s.True(s.redis.Forget("test-forget"))
}

func (s *CacheTestSuite) TestFlush() {
	s.Nil(s.redis.Put("test-flush", "goravel", 5*time.Second))
	s.Equal("goravel", s.redis.Get("test-flush", nil).(string))

	s.True(s.redis.Flush())
	s.False(s.redis.Has("test-flush"))
}

func (s *CacheTestSuite) TestGet() {
	s.Nil(s.redis.Put("name", "Goravel", 1*time.Second))
	s.Equal("Goravel", s.redis.Get("name", "").(string))
	s.Equal("World", s.redis.Get("name1", "World").(string))
	s.Equal("World1", s.redis.Get("name2", func() any {
		return "World1"
	}).(string))
	s.True(s.redis.Forget("name"))
	s.True(s.redis.Flush())
}

func (s *CacheTestSuite) TestGetBool() {
	s.Equal(true, s.redis.GetBool("test-get-bool", true))
	s.Nil(s.redis.Put("test-get-bool", true, 2*time.Second))
	s.Equal(true, s.redis.GetBool("test-get-bool", false))
}

func (s *CacheTestSuite) TestGetInt() {
	s.Equal(2, s.redis.GetInt("test-get-int", 2))
	s.Nil(s.redis.Put("test-get-int", 3, 2*time.Second))
	s.Equal(3, s.redis.GetInt("test-get-int", 2))
}

func (s *CacheTestSuite) TestGetString() {
	s.Equal("2", s.redis.GetString("test-get-string", "2"))
	s.Nil(s.redis.Put("test-get-string", "3", 2*time.Second))
	s.Equal("3", s.redis.GetString("test-get-string", "2"))
}

func (s *CacheTestSuite) TestHas() {
	s.False(s.redis.Has("test-has"))
	s.Nil(s.redis.Put("test-has", "goravel", 5*time.Second))
	s.True(s.redis.Has("test-has"))
}

func (s *CacheTestSuite) TestIncrement() {
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

func (s *CacheTestSuite) TestLock() {
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

func (s *CacheTestSuite) TestPull() {
	s.Nil(s.redis.Put("name", "Goravel", 1*time.Second))
	s.True(s.redis.Has("name"))
	s.Equal("Goravel", s.redis.Pull("name", "").(string))
	s.False(s.redis.Has("name"))
}

func (s *CacheTestSuite) TestPut() {
	s.Nil(s.redis.Put("name", "Goravel", 1*time.Second))
	s.True(s.redis.Has("name"))
	s.Equal("Goravel", s.redis.Get("name", "").(string))
	time.Sleep(2 * time.Second)
	s.False(s.redis.Has("name"))
}

func (s *CacheTestSuite) TestRemember() {
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

func (s *CacheTestSuite) TestRememberForever() {
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
