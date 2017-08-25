// This file was automatically generated by genx.
// Any changes will be lost if this file is regenerated.
// see https://github.com/OneOfOne/genx
// cmd: genx -pkg github.com/OneOfOne/cmap -v -t KT=interface{},VT=interface{} -o ./cmap_iface_iface.go
// +build !genx

package cmap

import (
	"context"
	"sync"

	"github.com/OneOfOne/cmap/hashers"
)

// DefaultShardCount is the default number of shards to use when New() or NewFromJSON() are called. The default is 256.
const DefaultShardCount = 1 << 8

// CMap is a concurrent safe sharded map to scale on multiple cores.
type CMap struct {
	shards   []*LMap
	keysPool sync.Pool
}

// New is an alias for NewSize(DefaultShardCount)
func New() *CMap { return NewSize(DefaultShardCount) }

// NewSize returns a CMap with the specific shardSize, note that for performance reasons,
// shardCount must be a power of 2.
// Higher shardCount will improve concurrency but will consume more memory.
func NewSize(shardCount int) *CMap {
	// must be a power of 2
	if shardCount < 1 {
		shardCount = DefaultShardCount
	} else if shardCount&(shardCount-1) != 0 {
		panic("shardCount must be a power of 2")
	}

	cm := &CMap{
		shards: make([]*LMap, shardCount),
	}

	cm.keysPool.New = func() interface{} {
		out := make([]interface{}, 0, DefaultShardCount) // good starting round

		return &out // return a ptr to avoid extra allocation on Get/Put
	}

	for i := range cm.shards {
		cm.shards[i] = NewLMapSize(shardCount)
	}

	return cm
}

// ShardForKey returns the LMap that may hold the specific key.
func (cm *CMap) ShardForKey(key interface{}) *LMap {
	h := hasher(key)
	return cm.shards[h&uint32(len(cm.shards)-1)]
}

// Set is the equivalent of `map[key] = val`.
func (cm *CMap) Set(key interface{}, val interface{}) {
	h := hasher(key)
	cm.shards[h&uint32(len(cm.shards)-1)].Set(key, val)
}

// SetIfNotExists will only assign val to key if it wasn't already set.
// Use `Update` if you need more logic.
func (cm *CMap) SetIfNotExists(key interface{}, val interface{}) (set bool) {
	h := hasher(key)
	return cm.shards[h&uint32(len(cm.shards)-1)].SetIfNotExists(key, val)
}

// Get is the equivalent of `val := map[key]`.
func (cm *CMap) Get(key interface{}) (val interface{}) {
	h := hasher(key)
	return cm.shards[h&uint32(len(cm.shards)-1)].Get(key)
}

// GetOK is the equivalent of `val, ok := map[key]`.
func (cm *CMap) GetOK(key interface{}) (val interface{}, ok bool) {
	h := hasher(key)
	return cm.shards[h&uint32(len(cm.shards)-1)].GetOK(key)
}

// Has is the equivalent of `_, ok := map[key]`.
func (cm *CMap) Has(key interface{}) bool {
	h := hasher(key)
	return cm.shards[h&uint32(len(cm.shards)-1)].Has(key)
}

// Delete is the equivalent of `delete(map, key)`.
func (cm *CMap) Delete(key interface{}) {
	h := hasher(key)
	cm.shards[h&uint32(len(cm.shards)-1)].Delete(key)
}

// DeleteAndGet is the equivalent of `oldVal := map[key]; delete(map, key)`.
func (cm *CMap) DeleteAndGet(key interface{}) interface{} {
	h := hasher(key)
	return cm.shards[h&uint32(len(cm.shards)-1)].DeleteAndGet(key)
}

// Update calls `fn` with the key's old value (or nil) and assign the returned value to the key.
// The shard containing the key will be locked, it is NOT safe to call other cmap funcs inside `fn`.
func (cm *CMap) Update(key interface{}, fn func(oldval interface{}) (newval interface{})) {
	h := hasher(key)
	cm.shards[h&uint32(len(cm.shards)-1)].Update(key, fn)
}

// Swap is the equivalent of `oldVal, map[key] = map[key], newVal`.
func (cm *CMap) Swap(key interface{}, val interface{}) interface{} {
	h := hasher(key)
	return cm.shards[h&uint32(len(cm.shards)-1)].Swap(key, val)
}

// Keys returns a slice of all the keys of the map.
func (cm *CMap) Keys() []interface{} {
	out := make([]interface{}, 0, cm.Len())
	for _, sh := range cm.shards {
		out = sh.Keys(out)
	}
	return out
}

// ForEach loops over all the key/values in the map.
// You can break early by returning false.
// It **is** safe to modify the map while using this iterator, however it uses more memory and is slightly slower.
func (cm *CMap) ForEach(fn func(key interface{}, val interface{}) bool) bool {
	keysP := cm.keysPool.Get().(*[]interface{})
	defer cm.keysPool.Put(keysP)

	for _, lm := range cm.shards {
		keys := (*keysP)[:0]
		if !lm.ForEach(keys, fn) {
			return false
		}
	}

	return false
}

// ForEachLocked loops over all the key/values in the map.
// You can break early by returning false.
// It is **NOT* safe to modify the map while using this iterator.
func (cm *CMap) ForEachLocked(fn func(key interface{}, val interface{}) bool) bool {
	for _, lm := range cm.shards {
		if !lm.ForEachLocked(fn) {
			return false
		}
	}

	return true
}

// Len returns the length of the map.
func (cm *CMap) Len() int {
	ln := 0
	for _, lm := range cm.shards {
		ln += lm.Len()
	}
	return ln
}

// ShardDistribution returns the distribution of data amoung all shards.
// Useful for debugging the efficiency of a hash.
func (cm *CMap) ShardDistribution() []float64 {
	var (
		out = make([]float64, len(cm.shards))
		ln  = float64(cm.Len())
	)
	for i := range out {
		out[i] = float64(cm.shards[i].Len()) / ln
	}
	return out
}

// KV holds the key/value returned when Iter is called.
type KV struct {
	Key   interface{}
	Value interface{}
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

// iterContext is used internally
func (cm *CMap) iterContext(ctx context.Context, ch chan<- *KV, locked bool) {
	fn := func(k interface{}, v interface{}) bool {
		select {
		case <-ctx.Done():
			return false
		case ch <- &KV{k, v}:
			return true
		}
	}

	if locked {
		_ = cm.ForEachLocked(fn)
	} else {
		_ = cm.ForEach(fn)
	}
}

// NumShards returns the number of shards in the map.
func (cm *CMap) NumShards() int { return len(cm.shards) }

func hasher(key interface{}) uint32 { return hashers.TypeHasher32(key) }

// LMap is a simple sync.RWMutex locked map.
// Used by CMap internally for sharding.
type LMap struct {
	m map[interface{}]interface{}
	l *sync.RWMutex
}

// NewLMap returns a new LMap with the cap set to 0.
func NewLMap() *LMap {
	return NewLMapSize(0)
}

// NewLMapSize is the equivalent of `m := make(map[interface{}]interface{}, cap)`
func NewLMapSize(cap int) *LMap {
	return &LMap{
		m: make(map[interface{}]interface{}, cap),
		l: new(sync.RWMutex),
	}
}

// Set is the equivalent of `map[key] = val`.
func (lm *LMap) Set(key interface{}, v interface{}) {
	lm.l.Lock()
	lm.m[key] = v
	lm.l.Unlock()
}

// SetIfNotExists will only assign val to key if it wasn't already set.
// Use `Update` if you need more logic.
func (lm *LMap) SetIfNotExists(key interface{}, val interface{}) (set bool) {
	lm.l.Lock()
	if _, ok := lm.m[key]; !ok {
		lm.m[key], set = val, true
	}
	lm.l.Unlock()
	return
}

// Get is the equivalent of `val := map[key]`.
func (lm *LMap) Get(key interface{}) (v interface{}) {
	lm.l.RLock()
	v = lm.m[key]
	lm.l.RUnlock()
	return
}

// GetOK is the equivalent of `val, ok := map[key]`.
func (lm *LMap) GetOK(key interface{}) (v interface{}, ok bool) {
	lm.l.RLock()
	v, ok = lm.m[key]
	lm.l.RUnlock()
	return
}

// Has is the equivalent of `_, ok := map[key]`.
func (lm *LMap) Has(key interface{}) (ok bool) {
	lm.l.RLock()
	_, ok = lm.m[key]
	lm.l.RUnlock()
	return
}

// Delete is the equivalent of `delete(map, key)`.
func (lm *LMap) Delete(key interface{}) {
	lm.l.Lock()
	delete(lm.m, key)
	lm.l.Unlock()
}

// DeleteAndGet is the equivalent of `oldVal := map[key]; delete(map, key)`.
func (lm *LMap) DeleteAndGet(key interface{}) (v interface{}) {
	lm.l.Lock()
	v = lm.m[key]
	delete(lm.m, key)
	lm.l.Unlock()
	return v
}

// Update calls `fn` with the key's old value (or nil) and assigns the returned value to the key.
// The shard containing the key will be locked, it is NOT safe to call other cmap funcs inside `fn`.
func (lm *LMap) Update(key interface{}, fn func(oldVal interface{}) (newVal interface{})) {
	lm.l.Lock()
	lm.m[key] = fn(lm.m[key])
	lm.l.Unlock()
}

// Swap is the equivalent of `oldVal, map[key] = map[key], newVal`.
func (lm *LMap) Swap(key interface{}, newV interface{}) (oldV interface{}) {
	lm.l.Lock()
	oldV = lm.m[key]
	lm.m[key] = newV
	lm.l.Unlock()
	return
}

// ForEach loops over all the key/values in the map.
// You can break early by returning an error .
// It **is** safe to modify the map while using this iterator, however it uses more memory and is slightly slower.
func (lm *LMap) ForEach(keys []interface{}, fn func(key interface{}, val interface{}) bool) bool {
	lm.l.RLock()
	for key := range lm.m {
		keys = append(keys, key)
	}
	lm.l.RUnlock()

	for _, key := range keys {
		lm.l.RLock()
		val, ok := lm.m[key]
		lm.l.RUnlock()
		if !ok {
			continue
		}
		if !fn(key, val) {
			return false
		}
	}

	return true
}

// ForEachLocked loops over all the key/values in the map.
// You can break early by returning false
// It is **NOT* safe to modify the map while using this iterator.
func (lm *LMap) ForEachLocked(fn func(key interface{}, val interface{}) bool) bool {
	lm.l.RLock()
	defer lm.l.RUnlock()

	for key, val := range lm.m {
		if !fn(key, val) {
			return false
		}
	}

	return true
}

// Len returns the length of the map.
func (lm *LMap) Len() (ln int) {
	lm.l.RLock()
	ln = len(lm.m)
	lm.l.RUnlock()
	return
}

// Keys appends all the keys in the map to buf and returns buf.
// buf may be nil.
func (lm *LMap) Keys(buf []interface{}) []interface{} {
	lm.l.RLock()
	if cap(buf) == 0 {
		buf = make([]interface{}, 0, len(lm.m))
	}
	for k := range lm.m {
		buf = append(buf, k)
	}
	lm.l.RUnlock()
	return buf
}
