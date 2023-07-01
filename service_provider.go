package redis

import (
	"context"

	"github.com/goravel/framework/contracts/foundation"
)

const Binding = "goravel.redis"

var App foundation.Application

type ServiceProvider struct {
}

func (receiver *ServiceProvider) Register(app foundation.Application) {
	App = app

	app.BindWith(Binding, func(app foundation.Application, parameters map[string]any) (any, error) {
		return NewRedis(context.Background(), app.MakeConfig(), parameters["store"].(string))
	})
}

func (receiver *ServiceProvider) Boot(app foundation.Application) {

}
