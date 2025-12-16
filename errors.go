package redis

import "github.com/goravel/framework/errors"

var (
	ErrRedisServiceProviderNotRegistered = errors.New("please register redis service provider").SetModule("Redis")
	ErrRedisStoreIsRequired              = errors.New("store is required").SetModule("Redis")
	ErrRedisConnectionIsRequired         = errors.New("connection is required").SetModule("Redis")
	ErrSessionDriverIsRequired           = errors.New("session driver is required").SetModule("Redis")
)
