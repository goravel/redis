package redis

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func (s *CacheTestSuite) TestTagsPutAndGet() {
	s.True(s.cache.Flush())

	s.NoError(s.cache.Tags("people", "authors").Put("john", "John", time.Minute))

	s.Equal("John", s.cache.Tags("people", "authors").Get("john"))
	s.Equal("John", s.cache.Tags("people", "authors").GetString("john"))
	s.True(s.cache.Tags("people", "authors").Has("john"))

	// Items are namespaced by their tags, in order, like Laravel.
	s.Nil(s.cache.Get("john"))
	s.Nil(s.cache.Tags("people").Get("john"))
	s.Nil(s.cache.Tags("authors", "people").Get("john"))
	s.Equal("default", s.cache.Tags("people").Get("john", "default"))
}

func (s *CacheTestSuite) TestTagsTypedGetters() {
	s.True(s.cache.Flush())

	tagged := s.cache.Tags("typed")
	s.NoError(tagged.Put("bool", true, time.Minute))
	s.NoError(tagged.Put("int", 3, time.Minute))

	s.True(tagged.GetBool("bool"))
	s.Equal(3, tagged.GetInt("int"))
	s.Equal(int64(3), tagged.GetInt64("int"))
	s.Equal("3", tagged.GetString("int"))
	s.Equal(2, tagged.GetInt("missing", 2))
}

func (s *CacheTestSuite) TestTagsSameKeyInDifferentTags() {
	s.True(s.cache.Flush())

	s.NoError(s.cache.Tags("a").Put("key", "a", time.Minute))
	s.NoError(s.cache.Tags("b").Put("key", "b", time.Minute))
	s.NoError(s.cache.Put("key", "plain", time.Minute))

	s.Equal("a", s.cache.Tags("a").Get("key"))
	s.Equal("b", s.cache.Tags("b").Get("key"))
	s.Equal("plain", s.cache.Get("key"))
}

func (s *CacheTestSuite) TestTagsFlushRemovesItemsOfAnyTag() {
	s.True(s.cache.Flush())

	s.NoError(s.cache.Tags("people", "authors").Put("john", "John", time.Minute))
	s.True(s.cache.Tags("people").Forever("anne", "Anne"))
	s.True(s.cache.Tags("authors").Forever("bob", "Bob"))
	s.True(s.cache.Tags("other").Forever("tom", "Tom"))
	s.True(s.cache.Forever("plain", "Plain"))

	s.True(s.cache.Tags("people").Flush())

	s.Nil(s.cache.Tags("people", "authors").Get("john"))
	s.Nil(s.cache.Tags("people").Get("anne"))
	s.Equal("Bob", s.cache.Tags("authors").Get("bob"))
	s.Equal("Tom", s.cache.Tags("other").Get("tom"))
	s.Equal("Plain", s.cache.Get("plain"))
	s.Equal(int64(0), s.cache.instance.Exists(s.ctx(), s.tagKey("people")).Val())

	// The flushed item is dropped from its other tags too, together with its tag list.
	authors, err := s.cache.instance.ZRange(s.ctx(), s.tagKey("authors"), 0, -1).Result()
	s.NoError(err)
	s.Equal([]string{s.cache.key(s.cache.Tags("authors").(*taggedCache).itemKey("bob"))}, authors)
	tagged := s.cache.Tags("people", "authors").(*taggedCache)
	s.Equal(int64(0), s.cache.instance.Exists(s.ctx(), tagged.namesKey(tagged.namespace)).Val())

	// The flushed tag can be used again.
	s.True(s.cache.Tags("people").Forever("anne", "Anne"))
	s.Equal("Anne", s.cache.Tags("people").Get("anne"))
}

func (s *CacheTestSuite) TestTagsIndexDoesNotCollideWithApplicationKeys() {
	s.True(s.cache.Flush())

	s.True(s.cache.Forever("tag:people:entries", "app"))
	s.True(s.cache.Tags("people").Forever("john", "John"))
	s.True(s.cache.Tags("people").Flush())

	s.Nil(s.cache.Tags("people").Get("john"))
	s.Equal("app", s.cache.Get("tag:people:entries"))
}

func (s *CacheTestSuite) TestTagsFlushInChunks() {
	s.True(s.cache.Flush())

	count := tagFlushChunk + 5
	for i := 0; i < count; i++ {
		s.True(s.cache.Tags("many").Forever(strconv.Itoa(i), i))
	}

	s.True(s.cache.Tags("many").Flush())
	for _, i := range []int{0, tagFlushChunk - 1, tagFlushChunk, count - 1} {
		s.False(s.cache.Tags("many").Has(strconv.Itoa(i)))
	}
}

func (s *CacheTestSuite) TestTagsFlushOfUnknownTag() {
	s.True(s.cache.Tags("unknown").Flush())
	s.True(s.cache.Tags().Flush())
}

func (s *CacheTestSuite) TestTagsWithoutNames() {
	s.True(s.cache.Flush())

	s.True(s.cache.Tags().Forever("key", "value"))
	s.Equal("value", s.cache.Tags().Get("key"))
	s.Nil(s.cache.Get("key"))

	// No tag indexes the item, so a flush of the empty set removes nothing.
	s.True(s.cache.Tags().Flush())
	s.Equal("value", s.cache.Tags().Get("key"))
}

func (s *CacheTestSuite) TestTagsNamesAreLengthPrefixed() {
	s.True(s.cache.Flush())

	s.True(s.cache.Tags("ab").Forever("key", "joined"))
	s.True(s.cache.Tags("a", "b").Forever("key", "split"))

	s.Equal("joined", s.cache.Tags("ab").Get("key"))
	s.Equal("split", s.cache.Tags("a", "b").Get("key"))

	s.True(s.cache.Tags("a").Flush())
	s.Equal("joined", s.cache.Tags("ab").Get("key"))
	s.Nil(s.cache.Tags("a", "b").Get("key"))
}

func (s *CacheTestSuite) TestTagsNamesAreCopied() {
	s.True(s.cache.Flush())

	names := []string{"a", "b"}
	tagged := s.cache.Tags(names...)
	names[0] = "c"

	s.True(tagged.Forever("key", "value"))
	s.Equal("value", s.cache.Tags("a", "b").Get("key"))

	s.True(s.cache.Tags("a").Flush())
	s.Nil(tagged.Get("key"))
}

func (s *CacheTestSuite) TestTagsAdd() {
	s.True(s.cache.Flush())

	s.True(s.cache.Tags("a").Add("key", "first", time.Minute))
	s.False(s.cache.Tags("a").Add("key", "second", time.Minute))
	s.Equal("first", s.cache.Tags("a").Get("key"))

	s.True(s.cache.Tags("a").Flush())
	s.False(s.cache.Tags("a").Has("key"))
	s.True(s.cache.Tags("a").Add("key", "third", time.Minute))
	s.Equal("third", s.cache.Tags("a").Get("key"))
}

func (s *CacheTestSuite) TestTagsIncrementAndDecrement() {
	s.True(s.cache.Flush())

	res, err := s.cache.Tags("counters").Increment("visits")
	s.NoError(err)
	s.Equal(int64(1), res)

	res, err = s.cache.Tags("counters").Increment("visits", 4)
	s.NoError(err)
	s.Equal(int64(5), res)

	res, err = s.cache.Tags("counters").Decrement("visits", 2)
	s.NoError(err)
	s.Equal(int64(3), res)
	s.Equal(int64(3), s.cache.Tags("counters").GetInt64("visits"))

	s.True(s.cache.Tags("counters").Flush())
	s.False(s.cache.Tags("counters").Has("visits"))

	res, err = s.cache.Tags("counters").Decrement("visits")
	s.NoError(err)
	s.Equal(int64(-1), res)

	s.NoError(s.cache.Tags("counters").Put("name", "goravel", time.Minute))
	_, err = s.cache.Tags("counters").Increment("name")
	s.Error(err)
}

func (s *CacheTestSuite) TestTagsForgetAndPull() {
	s.True(s.cache.Flush())

	s.True(s.cache.Tags("a", "b").Forever("key", "value"))
	s.True(s.cache.Tags("a", "b").Forget("key"))
	s.False(s.cache.Tags("a", "b").Has("key"))
	s.Equal(int64(0), s.cache.instance.ZCard(s.ctx(), s.tagKey("a")).Val())
	s.Equal(int64(0), s.cache.instance.ZCard(s.ctx(), s.tagKey("b")).Val())

	s.True(s.cache.Tags("a").Forever("key", "value"))
	s.Equal("value", s.cache.Tags("a").Pull("key"))
	s.False(s.cache.Tags("a").Has("key"))
	s.Equal("default", s.cache.Tags("a").Pull("key", "default"))
}

func (s *CacheTestSuite) TestTagsExpiration() {
	s.True(s.cache.Flush())

	s.NoError(s.cache.Tags("a").Put("key", "value", time.Second))
	s.True(s.cache.Tags("a").Has("key"))
	time.Sleep(2 * time.Second)
	s.False(s.cache.Tags("a").Has("key"))

	// The next write drops the stale entry, a forever one stays.
	s.True(s.cache.Tags("a").Forever("other", "value"))
	entries, err := s.cache.instance.ZRangeWithScores(s.ctx(), s.tagKey("a"), 0, -1).Result()
	s.NoError(err)
	s.Len(entries, 1)
	s.Equal(s.cache.key(s.cache.Tags("a").(*taggedCache).itemKey("other")), entries[0].Member)
	s.Equal(float64(foreverScore), entries[0].Score)
}

func (s *CacheTestSuite) TestTagsRemember() {
	s.True(s.cache.Flush())

	value, err := s.cache.Tags("a").Remember("key", time.Minute, func() (any, error) {
		return "first", nil
	})
	s.NoError(err)
	s.Equal("first", value)

	value, err = s.cache.Tags("a").Remember("key", time.Minute, func() (any, error) {
		return "second", nil
	})
	s.NoError(err)
	s.Equal("first", value)

	_, err = s.cache.Tags("a").Remember("failed", time.Minute, func() (any, error) {
		return nil, errors.New("error")
	})
	s.EqualError(err, "error")
	s.False(s.cache.Tags("a").Has("failed"))

	value, err = s.cache.Tags("b").RememberForever("key", func() (any, error) {
		return "forever", nil
	})
	s.NoError(err)
	s.Equal("forever", value)

	s.True(s.cache.Tags("a").Flush())
	s.False(s.cache.Tags("a").Has("key"))
	s.True(s.cache.Tags("b").Has("key"))
}

func (s *CacheTestSuite) TestTagsStoreFlush() {
	s.True(s.cache.Tags("a").Forever("key", "value"))
	s.True(s.cache.Flush())

	s.False(s.cache.Tags("a").Has("key"))
	s.Equal(int64(0), s.cache.instance.Exists(s.ctx(), s.tagKey("a")).Val())
}

func (s *CacheTestSuite) ctx() context.Context {
	return s.cache.ctx
}

func (s *CacheTestSuite) tagKey(name string) string {
	return s.cache.Tags().(*taggedCache).tagKey(name)
}

func TestTagNamespace(t *testing.T) {
	assert.Equal(t, tagNamespace([]string{"a", "b"}), tagNamespace([]string{"a", "b"}))
	assert.NotEqual(t, tagNamespace([]string{"a", "b"}), tagNamespace([]string{"b", "a"}))
	assert.NotEqual(t, tagNamespace([]string{"ab"}), tagNamespace([]string{"a", "b"}))
	assert.NotEqual(t, tagNamespace([]string{"a|b"}), tagNamespace([]string{"a", "b"}))
	assert.Len(t, tagNamespace(nil), 40)
}

func TestTagsKeys(t *testing.T) {
	cache := &Cache{prefix: "goravel_cache"}
	tagged := cache.Tags("people").(*taggedCache)

	assert.Equal(t, tagNamespace([]string{"people"})+":john", tagged.itemKey("john"))
	assert.Equal(t, "goravel_cache:tag:465d4ae24cac06daecf97e3b106cfbc34559b7e7:entries", tagged.tagKey("people"))
	assert.Equal(t, "goravel_cache:tag:465d4ae24cac06daecf97e3b106cfbc34559b7e7:names", tagged.namesKey(tagged.namespace))
	assert.Equal(t, tagged.namespace, tagged.namespaceOf(cache.key(tagged.itemKey("john:doe"))))
	assert.Equal(t, "", tagged.namespaceOf(cache.key("plain")))
	assert.Equal(t, "", tagged.namespaceOf("other:"+tagged.itemKey("john")))
}
