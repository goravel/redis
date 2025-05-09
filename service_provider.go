package redis

import (
	"context"

	"github.com/goravel/framework/contracts/foundation"
	"github.com/goravel/framework/errors"
)

const (
	CacheBinding   = "goravel.redis.cache"
	QueueBinding   = "goravel.redis.queue"
	SessionBinding = "goravel.redis.session"

	Name = "redis"
)

var App foundation.Application

type ServiceProvider struct {
}

func (receiver *ServiceProvider) Register(app foundation.Application) {
	App = app

	app.BindWith(CacheBinding, func(app foundation.Application, parameters map[string]any) (any, error) {
		config := app.MakeConfig()
		if config == nil {
			return nil, errors.ConfigFacadeNotSet.SetModule(errors.ModuleCache)
		}

		return NewCache(context.Background(), config, parameters["store"].(string))
	})

	app.BindWith(QueueBinding, func(app foundation.Application, parameters map[string]any) (any, error) {
		config := app.MakeConfig()
		if config == nil {
			return nil, errors.ConfigFacadeNotSet.SetModule(errors.ModuleQueue)
		}

		queue := app.MakeQueue()
		if queue == nil {
			return nil, errors.QueueFacadeNotSet.SetModule(errors.ModuleQueue)
		}

		return NewQueue(context.Background(), config, queue, app.GetJson(), parameters["connection"].(string))
	})
	app.BindWith(SessionBinding, func(app foundation.Application, parameters map[string]any) (any, error) {
		return NewSession(context.Background(), app.MakeConfig(), parameters["driver"].(string))
	})
}

func (receiver *ServiceProvider) Boot(app foundation.Application) {

}
