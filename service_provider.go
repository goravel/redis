package redis

import (
	"context"

	"github.com/goravel/framework/contracts/binding"
	"github.com/goravel/framework/contracts/foundation"
	"github.com/goravel/framework/errors"
)

const (
	BindingCache   = "goravel.redis.cache"
	BindingQueue   = "goravel.redis.queue"
	BindingSession = "goravel.redis.session"

	Name = "redis"
)

var App foundation.Application

type ServiceProvider struct {
}

func (r *ServiceProvider) Relationship() binding.Relationship {
	return binding.Relationship{
		Bindings: []string{
			BindingCache,
			BindingQueue,
			BindingSession,
		},
		Dependencies: []string{
			binding.Config,
		},
		ProvideFor: []string{
			binding.Cache,
			binding.Queue,
			binding.Session,
		},
	}
}

func (r *ServiceProvider) Register(app foundation.Application) {
	App = app

	app.BindWith(BindingCache, func(app foundation.Application, parameters map[string]any) (any, error) {
		config := app.MakeConfig()
		if config == nil {
			return nil, errors.ConfigFacadeNotSet.SetModule(errors.ModuleCache)
		}

		return NewCache(context.Background(), config, app.MakeProcess(), parameters["store"].(string))
	})
	app.BindWith(BindingQueue, func(app foundation.Application, parameters map[string]any) (any, error) {
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
	app.BindWith(BindingSession, func(app foundation.Application, parameters map[string]any) (any, error) {
		return NewSession(context.Background(), app.MakeConfig(), parameters["driver"].(string))
	})
}

func (r *ServiceProvider) Boot(app foundation.Application) {

}
