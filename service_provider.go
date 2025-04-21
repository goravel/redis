package redis

import (
	"context"

	"github.com/goravel/framework/contracts/foundation"
)

const (
	CacheBinding   = "goravel.redis.cache"
	QueueBinding   = "goravel.redis.queue"
	SessionBinding = "goravel.redis.session"
)

var App foundation.Application

type ServiceProvider struct {
}

func (receiver *ServiceProvider) Register(app foundation.Application) {
	App = app

	app.BindWith(CacheBinding, func(app foundation.Application, parameters map[string]any) (any, error) {
		return NewCache(context.Background(), app.MakeConfig(), parameters["store"].(string))
	})
	app.BindWith(QueueBinding, func(app foundation.Application, parameters map[string]any) (any, error) {
		return NewQueue(context.Background(), app.MakeConfig(), app.MakeQueue(), parameters["connection"].(string))
	})
	app.BindWith(SessionBinding, func(app foundation.Application, parameters map[string]any) (any, error) {
		return NewSession(context.Background(), app.MakeConfig(), parameters["driver"].(string))
	})
}

func (receiver *ServiceProvider) Boot(app foundation.Application) {

}
