package cmap

import (
	"context"
	"sync"

	"github.com/OneOfOne/cmap"
)

var Break = cmap.Break

type KV struct {
	Key   KT
	Value VT
}

// CMap is a concurrent safe sharded map to scale on multiple cores.
type CMap struct {
	shards []*LockedMap

	// HashFn allows using a custom hash function that's used to determain the key's shard.
	// Defaults to DefaultKeyHasher.
	HashFn func(KT) uint32

	keysPool sync.Pool
	mod      uint32
}

// New is an alias for NewSize(DefaultShardCount)
func New() *CMap { return NewSize(cmap.DefaultShardCount) }

// NewSize returns a CMap with the specific shardSize, note that for performance reasons,
// shardCount must be a power of 2.
// Higher shardCount will improve concurrency but will consume more memory.
func NewSize(shardCount int) *CMap {
	// must be a power of 2
	if shardCount < 1 {
		shardCount = cmap.DefaultShardCount
	} else if shardCount&(shardCount-1) != 0 {
		panic("shardCount must be a power of 2")
	}

	cm := &CMap{
		shards: make([]*LockedMap, shardCount),
		mod:    uint32(shardCount) - 1,
		HashFn: DefaultKeyHasher,
	}

	for i := range cm.shards {
		cm.shards[i] = NewLockedMapSize(shardCount)
	}

	cm.keysPool.New = func() interface{} {
		out := make([]KT, 0, shardCount) // good starting round

		return &out // return a ptr to avoid extra allocation on Get/Put
	}

	return cm
}

func (cm *CMap) shard(key KT) *LockedMap {
	h := cm.HashFn(key)
	return cm.shards[h&cm.mod]
}

// Get is the equivalent of `val := map[key]`.
func (cm *CMap) Get(key KT) (val VT) {
	return cm.shard(key).Get(key)
}

// GetOK is the equivalent of `val, ok := map[key]`.
func (cm *CMap) GetOK(key KT) (val VT, ok bool) {
	return cm.shard(key).GetOK(key)
}

// Set is the equivalent of `map[key] = val`.
func (cm *CMap) Set(key KT, val VT) {
	cm.shard(key).Set(key, val)
}

// SetIfNotExists will only assign val to key if it wasn't already set.
// Use `CMap.Update` if you need more logic.
func (cm *CMap) SetIfNotExists(key KT, val VT) (set bool) {
	sh := cm.shard(key)
	sh.l.Lock()
	if _, ok := sh.m[key]; !ok {
		sh.m[key], set = val, true
	}
	sh.l.Unlock()
	return
}

// Has is the equivalent of `_, ok := map[key]`.
func (cm *CMap) Has(key KT) bool { return cm.shard(key).Has(key) }

// Delete is the equivalent of `delete(map, key)`.
func (cm *CMap) Delete(key KT) { cm.shard(key).Delete(key) }

// DeleteAndGet is the equivalent of `oldVal := map[key]; delete(map, key)`.
func (cm *CMap) DeleteAndGet(key KT) VT { return cm.shard(key).DeleteAndGet(key) }

// Update calls `fn` with the key's old value (or nil if it didn't exist) and assign the returned value to the key.
// The shard containing the key will be locked, it is NOT safe to call other cmap funcs inside `fn`.
func (cm *CMap) Update(key KT, fn func(oldval VT) (newval VT)) {
	cm.shard(key).Update(key, fn)
}

// Swap is the equivalent of `oldVal, map[key] = map[key], newVal`.
func (cm *CMap) Swap(key KT, val VT) VT {
	return cm.shard(key).Swap(key, val)
}

// Keys returns a slice of all the keys of the map.
func (cm *CMap) Keys() []KT {
	out := make([]KT, 0, cm.Len())
	for i := range cm.shards {
		sh := cm.shards[i]
		sh.l.RLock()
		for k := range sh.m {
			out = append(out, k)
		}
		sh.l.RUnlock()
	}
	return out
}

// ForEach loops over all the key/values in all the shards in order.
// You can break early by returning an error or Break.
// It **is** safe to modify the map while using this iterator, however it uses more memory and is slightly slower.
func (cm *CMap) ForEach(fn func(key KT, val VT) error) error {
	keysP := cm.keysPool.Get().(*[]KT)

	defer cm.keysPool.Put(keysP)
	for i := range cm.shards {
		keys := (*keysP)[:0]
		if err := cm.shards[i].ForEach(keys, fn); err != nil {
			if err == Break {
				return nil
			}
			return err
		}
	}
	return nil
}

// ForEachLocked loops over all the key/values in the map.
// You can break early by returning an error or Break.
// It is **NOT* safe to modify the map while using this iterator.
func (cm *CMap) ForEachLocked(fn func(key KT, val VT) error) error {
	for i := range cm.shards {
		if err := cm.shards[i].ForEachLocked(fn); err != nil {
			if err == Break {
				return nil
			}
			return err
		}
	}
	return nil
}

// Iter returns a channel to be used in for range.
// Use `context.WithCancel` if you intend to break early or goroutines will leak.
// It **is** safe to modify the map while using this iterator, however it uses more memory and is slightly slower.
func (cm *CMap) Iter(ctx context.Context, buffer int) <-chan *KV {
	ch := make(chan *KV, buffer)
	go func() {
		cm.iterContext(ctx, ch, false)
		close(ch)
	}()
	return ch
}

// IterLocked returns a channel to be used in for range.
// Use `context.WithCancel` if you intend to break early or goroutines will leak and map access will deadlock.
// It is **NOT* safe to modify the map while using this iterator.
func (cm *CMap) IterLocked(ctx context.Context, buffer int) <-chan *KV {
	ch := make(chan *KV, buffer)
	go func() {
		cm.iterContext(ctx, ch, false)
		close(ch)
	}()
	return ch
}

func (cm *CMap) iterContext(ctx context.Context, ch chan<- *KV, locked bool) {
	fn := func(k KT, v VT) error {
		select {
		case <-ctx.Done():
			return Break
		case ch <- &KV{k, v}:
			return nil
		}
	}
	if locked {
		cm.ForEachLocked(fn)
	} else {
		cm.ForEach(fn)
	}
}

// Len returns the number of elements in the map.
func (cm *CMap) Len() int {
	ln := 0
	for i := range cm.shards {
		ln += cm.shards[i].Len()
	}
	return ln
}

// NumShards returns the number of shards in the map.
func (cm *CMap) NumShards() int { return len(cm.shards) }
