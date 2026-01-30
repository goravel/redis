package facades

import (
	"context"
	"fmt"
	"testing"

	"github.com/goravel/framework/foundation/json"
	mocksconfig "github.com/goravel/framework/mocks/config"
	mocksfoundation "github.com/goravel/framework/mocks/foundation"
	mocksqueue "github.com/goravel/framework/mocks/queue"
	"github.com/goravel/framework/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goravel/redis"
)

func TestMakingSessionWithRealImplementation(t *testing.T) {

	mockApp := mocksfoundation.NewApplication(t)
	mockConfig := mocksconfig.NewConfig(t)
	mockConfig.ExpectedCalls = nil

	connectionName := "session_default"

	mockConfig.On("GetString", fmt.Sprintf("session.drivers.%s.connection", connectionName), "default").Return(connectionName).Once()

	mockConfig.On("GetString", fmt.Sprintf("database.redis.%s.host", connectionName)).Return("localhost").Once()
	mockConfig.On("GetString", fmt.Sprintf("database.redis.%s.port", connectionName), "6379").Return("6379").Once()
	mockConfig.On("GetString", fmt.Sprintf("database.redis.%s.password", connectionName)).Return("").Once()
	mockConfig.On("GetString", fmt.Sprintf("database.redis.%s.username", connectionName)).Return("").Once()
	mockConfig.On("GetInt", fmt.Sprintf("database.redis.%s.database", connectionName), 0).Return(0).Once()
	mockConfig.On("GetBool", fmt.Sprintf("database.redis.%s.cluster", connectionName), false).Return(false).Once()
	mockConfig.On("Get", fmt.Sprintf("database.redis.%s.tls", connectionName)).Return(nil).Once()

	mockConfig.On("GetInt", "session.lifetime", 120).Return(120).Once()
	mockConfig.On("GetString", "session.cookie", "goravel_session").Return("goravel_session_test_redis").Once()

	originalApp := redis.App
	defer func() {
		redis.App = originalApp
	}()
	redis.App = mockApp

	realSessionsInstance, err := redis.NewSession(context.Background(), mockConfig, connectionName)
	if err != nil {
		t.Skip("Skipping test as real driver creation failed:", err)
		return
	}

	mockApp.EXPECT().MakeWith(redis.BindingSession, map[string]any{"driver": connectionName}).
		Return(realSessionsInstance, nil)

	driver, err := Session(connectionName)
	require.NoError(t, err)

	require.NotNil(t, driver)
	assert.Equal(t, realSessionsInstance, driver)

	if err := realSessionsInstance.Close(); err != nil {
		t.Logf("Warning: Failed to close Redis driver: %v", err)
	}
}

func TestMakingCacheWithRealImplementation(t *testing.T) {

	mockApp := mocksfoundation.NewApplication(t)
	mockConfig := mocksconfig.NewConfig(t)
	mockConfig.ExpectedCalls = nil

	connectionName := "cache_default"

	mockConfig.On("GetString", fmt.Sprintf("cache.stores.%s.connection", connectionName), "default").Return(connectionName).Once()

	mockConfig.On("GetString", fmt.Sprintf("database.redis.%s.host", connectionName)).Return("localhost").Once()
	mockConfig.On("GetString", fmt.Sprintf("database.redis.%s.port", connectionName), "6379").Return("6379").Once()
	mockConfig.On("GetString", fmt.Sprintf("database.redis.%s.password", connectionName)).Return("").Once()
	mockConfig.On("GetString", fmt.Sprintf("database.redis.%s.username", connectionName)).Return("").Once()
	mockConfig.On("GetInt", fmt.Sprintf("database.redis.%s.database", connectionName), 0).Return(0).Once()
	mockConfig.On("GetBool", fmt.Sprintf("database.redis.%s.cluster", connectionName), false).Return(false).Once()
	mockConfig.On("Get", fmt.Sprintf("database.redis.%s.tls", connectionName)).Return(nil).Once()

	mockConfig.On("GetString", "cache.prefix").Return("goravel_cache").Once()

	originalApp := redis.App
	defer func() {
		redis.App = originalApp
	}()
	redis.App = mockApp

	realCacheInstance, err := redis.NewCache(context.Background(), mockConfig, process.New(), connectionName)
	if err != nil {
		t.Skip("Skipping test as real driver creation failed:", err)
		return
	}

	mockApp.EXPECT().MakeWith(redis.BindingCache, map[string]any{"store": connectionName}).
		Return(realCacheInstance, nil)

	store, err := Cache(connectionName)
	require.NoError(t, err)

	require.NotNil(t, store)
	assert.Equal(t, realCacheInstance, store)
}

func TestMakingQueueWithRealImplementation(t *testing.T) {

	connectionName := "queue_default"
	mockApp := mocksfoundation.NewApplication(t)
	mockConfig := mocksconfig.NewConfig(t)
	mockConfig.EXPECT().GetString("app.name", "goravel").Return("goravel").Once()
	mockConfig.On("GetString", fmt.Sprintf("queue.connections.%s.connection", connectionName), "default").Return(connectionName).Once()
	mockConfig.On("GetString", fmt.Sprintf("database.redis.%s.host", connectionName)).Return("localhost").Once()
	mockConfig.On("GetString", fmt.Sprintf("database.redis.%s.port", connectionName), "6379").Return("6379").Once()
	mockConfig.On("GetString", fmt.Sprintf("database.redis.%s.password", connectionName)).Return("").Once()
	mockConfig.On("GetString", fmt.Sprintf("database.redis.%s.username", connectionName)).Return("").Once()
	mockConfig.On("GetInt", fmt.Sprintf("database.redis.%s.database", connectionName), 0).Return(0).Once()
	mockConfig.On("GetBool", fmt.Sprintf("database.redis.%s.cluster", connectionName), false).Return(false).Once()
	mockConfig.On("Get", fmt.Sprintf("database.redis.%s.tls", connectionName)).Return(nil).Once()

	originalApp := redis.App
	defer func() {
		redis.App = originalApp
	}()
	redis.App = mockApp

	mockQueue := mocksqueue.NewQueue(t)
	mockJobStorer := mocksqueue.NewJobStorer(t)
	mockQueue.EXPECT().JobStorer().Return(mockJobStorer)

	realQueueInstance, err := redis.NewQueue(context.Background(), mockConfig, mockQueue, json.New(), connectionName)
	if err != nil {
		t.Skip("Skipping test as real driver creation failed:", err)
		return
	}

	mockApp.EXPECT().MakeWith(redis.BindingQueue, map[string]any{"connection": connectionName}).
		Return(realQueueInstance, nil)

	store, err := Queue(connectionName)
	require.NoError(t, err)

	require.NotNil(t, store)
	assert.Equal(t, realQueueInstance, store)
}
