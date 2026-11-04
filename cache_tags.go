package redis

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
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
	// Marshaling a string slice cannot fail.
	encodedNames, _ := json.Marshal(names)

	return &taggedCache{
		cache:        r,
		names:        slices.Clone(names),
		namespace:    tagNamespace(names),
		encodedNames: string(encodedNames),
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

var _ contractscache.Repository = &taggedCache{}

// taggedCache is the tagged view of Cache returned by Tags.
type taggedCache struct {
	cache        *Cache
	names        []string
	namespace    string
	encodedNames string
}

func (r *taggedCache) Add(key string, value any, t time.Duration) bool {
	var added *redis.BoolCmd
	_, err := r.cache.instance.Pipelined(r.cache.ctx, func(pipe redis.Pipeliner) error {
		// Keep the entry of an item that already exists, its expiration does not change.
		// A new item has no live entry, since the expired ones are dropped first.
		r.register(pipe, key, t, true)
		added = pipe.SetNX(r.cache.ctx, r.cache.key(r.itemKey(key)), value, t)

		return nil
	})

	return err == nil && added.Val()
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

// Flush removes every item stored under any of the tags, and drops those items
// from the indexes of their other tags.
func (r *taggedCache) Flush() bool {
	for _, name := range r.names {
		tagKey := r.tagKey(name)
		// The tag lists of the namespaces met while flushing this tag.
		namespaces := make(map[string][]string)
		for start := int64(0); ; start += tagFlushChunk {
			itemKeys, err := r.cache.instance.ZRange(r.cache.ctx, tagKey, start, start+tagFlushChunk-1).Result()
			if err != nil {
				return false
			}
			if len(itemKeys) == 0 {
				break
			}

			if !r.loadNames(itemKeys, namespaces) {
				return false
			}

			// Unlink the keys one by one, a multi-key command fails on a cluster when they live in different slots.
			if _, err := r.cache.instance.Pipelined(r.cache.ctx, func(pipe redis.Pipeliner) error {
				for _, itemKey := range itemKeys {
					pipe.Unlink(r.cache.ctx, itemKey)
					for _, sibling := range namespaces[r.namespaceOf(itemKey)] {
						if sibling != name {
							pipe.ZRem(r.cache.ctx, r.tagKey(sibling), itemKey)
						}
					}
				}

				return nil
			}); err != nil {
				return false
			}

			// The set is not changed while it is read, so the next chunk starts right after this one.
			if len(itemKeys) < tagFlushChunk {
				break
			}
		}

		if _, err := r.cache.instance.Pipelined(r.cache.ctx, func(pipe redis.Pipeliner) error {
			pipe.Unlink(r.cache.ctx, tagKey)
			// Every item of these namespaces carries the flushed tag, so none of them is left.
			for namespace := range namespaces {
				pipe.Unlink(r.cache.ctx, r.namesKey(namespace))
			}

			return nil
		}); err != nil {
			return false
		}
	}

	return true
}

// loadNames reads the tag lists of the namespaces of the item keys that are not
// known yet. A namespace without a stored list has a single tag.
func (r *taggedCache) loadNames(itemKeys []string, namespaces map[string][]string) bool {
	cmds := make(map[string]*redis.StringCmd)
	if _, err := r.cache.instance.Pipelined(r.cache.ctx, func(pipe redis.Pipeliner) error {
		for _, itemKey := range itemKeys {
			namespace := r.namespaceOf(itemKey)
			if _, ok := namespaces[namespace]; ok || namespace == "" {
				continue
			}
			if _, ok := cmds[namespace]; !ok {
				cmds[namespace] = pipe.Get(r.cache.ctx, r.namesKey(namespace))
			}
		}

		return nil
	}); err != nil && !errors.Is(err, redis.Nil) {
		return false
	}

	for namespace, cmd := range cmds {
		raw, err := cmd.Result()
		if errors.Is(err, redis.Nil) {
			namespaces[namespace] = nil
			continue
		}
		if err != nil {
			return false
		}

		var names []string
		_ = json.Unmarshal([]byte(raw), &names)
		namespaces[namespace] = names
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
		// INCRBY keeps the TTL of an existing counter, so its tag entry keeps the
		// original score: rewriting it as forever (t = 0) would keep an expiring
		// counter in the index after it is gone. A new counter never expires.
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

// namesKey holds the tag list of a namespace, so a flush of one tag can find
// the other tags of an item. A single-tag namespace has none.
func (r *taggedCache) namesKey(namespace string) string {
	return r.cache.key("tag:" + namespace + ":names")
}

// namespaceOf returns the tag namespace of a full item key, or "" when the key
// does not have one.
func (r *taggedCache) namespaceOf(itemKey string) string {
	if r.cache.prefix != "" {
		itemKey = strings.TrimPrefix(itemKey, r.cache.prefix+":")
	}

	namespace, _, found := strings.Cut(itemKey, ":")
	if !found || len(namespace) != sha1.Size*2 {
		return ""
	}

	return namespace
}

// register queues the item key into the entries of every tag, scored by its
// expiration time, and drops the entries that have already expired.
func (r *taggedCache) register(pipe redis.Pipeliner, key string, t time.Duration, onlyNew bool) {
	now := time.Now()
	score := float64(foreverScore)
	if t > 0 {
		score = float64(now.Add(t).UnixMilli())
	}

	if len(r.names) > 1 {
		pipe.Set(r.cache.ctx, r.namesKey(r.namespace), r.encodedNames, 0)
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
