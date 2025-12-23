package cache

import (
	"container/list"
	"strings"
	"sync"
)

// Cache is a thread-safe LRU cache
type Cache struct {
	capacity  int
	items     map[string]*list.Element
	evictList *list.List
	lock      sync.RWMutex
}

type entry struct {
	key   string
	value string
}

// New creates a new LRU cache with the given capacity
func New(capacity int) *Cache {
	return &Cache{
		capacity:  capacity,
		items:     make(map[string]*list.Element),
		evictList: list.New(),
	}
}

// Get retrieves a value from the cache
func (c *Cache) Get(key string) (string, bool) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if ent, ok := c.items[key]; ok {
		c.evictList.MoveToFront(ent)
		return ent.Value.(*entry).value, true
	}
	return "", false
}

// Set adds a value to the cache
func (c *Cache) Set(key string, value string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if ent, ok := c.items[key]; ok {
		c.evictList.MoveToFront(ent)
		ent.Value.(*entry).value = value
		return
	}

	ent := &entry{key, value}
	element := c.evictList.PushFront(ent)
	c.items[key] = element

	if c.evictList.Len() > c.capacity {
		c.removeOldest()
	}
}

// Invalidate removes a key from the cache
func (c *Cache) Invalidate(key string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if ent, ok := c.items[key]; ok {
		c.removeElement(ent)
	}
}

// InvalidatePrefix removes all keys that start with the given prefix
func (c *Cache) InvalidatePrefix(prefix string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	for key, ent := range c.items {
		if strings.HasPrefix(key, prefix) {
			c.removeElement(ent)
		}
	}
}

// removeOldest removes the least recently used item
func (c *Cache) removeOldest() {
	ent := c.evictList.Back()
	if ent != nil {
		c.removeElement(ent)
	}
}

// removeElement removes a specific element from the cache
func (c *Cache) removeElement(e *list.Element) {
	c.evictList.Remove(e)
	kv := e.Value.(*entry)
	delete(c.items, kv.key)
}
