package redis

import (
	"crypto/sha1"
	"encoding/hex"
	"slices"
	"strconv"
	"time"

	contractscache "github.com/goravel/framework/contracts/cache"
	"github.com/redis/go-redis/v9"
)

// tagFlushChunk bounds how many entries of a tag are read and deleted per round trip.
const tagFlushChunk = 1000

// foreverScore marks a tag entry whose item never expires.
const foreverScore = -1

// Tags begins a tagged cache operation. Like Laravel, an item is namespaced by
// its tags, so it is only reachable through the same tags in the same order,
// while flushing any one of the tags removes it.
//
// Every tag keeps a sorted set of the keys stored under it, scored by their
// expiration time, so a flush only deletes the items of that tag.
func (r *Cache) Tags(names ...string) contractscache.Repository {
	return &taggedCache{
		cache:     r,
		names:     slices.Clone(names),
		namespace: tagNamespace(names),
	}
}

// tagNamespace builds the key namespace of a tag set, in the same way as the
// framework's memory driver. Every name is prefixed by its length, so no two
// different tag lists hash the same input.
func tagNamespace(names []string) string {
	hash := sha1.New()
	for _, name := range names {
		hash.Write([]byte(strconv.Itoa(len(name)) + ":" + name))
	}

	return hex.EncodeToString(hash.Sum(nil))
}

// taggedCache is the tagged view of Cache returned by Tags.
type taggedCache struct {
	cache     *Cache
	names     []string
	namespace string
}

func (r *taggedCache) Add(key string, value any, t time.Duration) bool {
	if !r.cache.Add(r.itemKey(key), value, t) {
		return false
	}

	_, err := r.cache.instance.Pipelined(r.cache.ctx, func(pipe redis.Pipeliner) error {
		r.register(pipe, key, t, false)

		return nil
	})

	return err == nil
}

func (r *taggedCache) Decrement(key string, value ...int64) (int64, error) {
	if len(value) == 0 {
		value = append(value, 1)
	}

	return r.Increment(key, -value[0])
}

func (r *taggedCache) Forever(key string, value any) bool {
	return r.Put(key, value, 0) == nil
}

func (r *taggedCache) Forget(key string) bool {
	itemKey := r.cache.key(r.itemKey(key))
	_, err := r.cache.instance.Pipelined(r.cache.ctx, func(pipe redis.Pipeliner) error {
		pipe.Del(r.cache.ctx, itemKey)
		for _, name := range r.names {
			pipe.ZRem(r.cache.ctx, r.tagKey(name), itemKey)
		}

		return nil
	})

	return err == nil
}

// Flush removes every item stored under any of the tags.
func (r *taggedCache) Flush() bool {
	for _, name := range r.names {
		tagKey := r.tagKey(name)
		for start := int64(0); ; start += tagFlushChunk {
			itemKeys, err := r.cache.instance.ZRange(r.cache.ctx, tagKey, start, start+tagFlushChunk-1).Result()
			if err != nil {
				return false
			}
			if len(itemKeys) == 0 {
				break
			}

			// Delete the keys one by one, a multi-key DEL fails on a cluster when they live in different slots.
			if _, err := r.cache.instance.Pipelined(r.cache.ctx, func(pipe redis.Pipeliner) error {
				for _, itemKey := range itemKeys {
					pipe.Del(r.cache.ctx, itemKey)
				}

				return nil
			}); err != nil {
				return false
			}

			if len(itemKeys) < tagFlushChunk {
				break
			}
		}

		if err := r.cache.instance.Del(r.cache.ctx, tagKey).Err(); err != nil {
			return false
		}
	}

	return true
}

func (r *taggedCache) Get(key string, def ...any) any {
	return r.cache.Get(r.itemKey(key), def...)
}

func (r *taggedCache) GetBool(key string, def ...bool) bool {
	return r.cache.GetBool(r.itemKey(key), def...)
}

func (r *taggedCache) GetInt(key string, def ...int) int {
	return r.cache.GetInt(r.itemKey(key), def...)
}

func (r *taggedCache) GetInt64(key string, def ...int64) int64 {
	return r.cache.GetInt64(r.itemKey(key), def...)
}

func (r *taggedCache) GetString(key string, def ...string) string {
	return r.cache.GetString(r.itemKey(key), def...)
}

func (r *taggedCache) Has(key string) bool {
	return r.cache.Has(r.itemKey(key))
}

func (r *taggedCache) Increment(key string, value ...int64) (int64, error) {
	if len(value) == 0 {
		value = append(value, 1)
	}

	var res *redis.IntCmd
	if _, err := r.cache.instance.Pipelined(r.cache.ctx, func(pipe redis.Pipeliner) error {
		// Keep the entry of an existing item, a counter keeps its expiration.
		r.register(pipe, key, 0, true)
		res = pipe.IncrBy(r.cache.ctx, r.cache.key(r.itemKey(key)), value[0])

		return nil
	}); err != nil {
		return 0, err
	}

	return res.Val(), nil
}

func (r *taggedCache) Put(key string, value any, t time.Duration) error {
	_, err := r.cache.instance.Pipelined(r.cache.ctx, func(pipe redis.Pipeliner) error {
		r.register(pipe, key, t, false)
		pipe.Set(r.cache.ctx, r.cache.key(r.itemKey(key)), value, t)

		return nil
	})

	return err
}

func (r *taggedCache) Pull(key string, def ...any) any {
	res := r.Get(key, def...)
	r.Forget(key)

	return res
}

func (r *taggedCache) Remember(key string, ttl time.Duration, callback func() (any, error)) (any, error) {
	if val := r.Get(key); val != nil {
		return val, nil
	}

	val, err := callback()
	if err != nil {
		return nil, err
	}

	if err := r.Put(key, val, ttl); err != nil {
		return nil, err
	}

	return val, nil
}

func (r *taggedCache) RememberForever(key string, callback func() (any, error)) (any, error) {
	return r.Remember(key, 0, callback)
}

// itemKey namespaces the key by the tags, the store adds its prefix on top.
func (r *taggedCache) itemKey(key string) string {
	return r.namespace + ":" + key
}

// tagKey is the sorted set holding the item keys stored under a tag. The name
// is hashed so the index cannot collide with an application key.
func (r *taggedCache) tagKey(name string) string {
	return r.cache.key("tag:" + tagNamespace([]string{name}) + ":entries")
}

// register queues the item key into the entries of every tag, scored by its
// expiration time, and drops the entries that have already expired.
func (r *taggedCache) register(pipe redis.Pipeliner, key string, t time.Duration, onlyNew bool) {
	now := time.Now()
	score := float64(foreverScore)
	if t > 0 {
		score = float64(now.Add(t).UnixMilli())
	}

	entry := redis.Z{Score: score, Member: r.cache.key(r.itemKey(key))}
	for _, name := range r.names {
		tagKey := r.tagKey(name)
		pipe.ZRemRangeByScore(r.cache.ctx, tagKey, "0", strconv.FormatInt(now.UnixMilli(), 10))
		if onlyNew {
			pipe.ZAddNX(r.cache.ctx, tagKey, entry)
		} else {
			pipe.ZAdd(r.cache.ctx, tagKey, entry)
		}
	}
}
