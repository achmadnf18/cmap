# cmap [![GoDoc](http://godoc.org/github.com/OneOfOne/cmap?status.svg)](http://godoc.org/github.com/OneOfOne/cmap) [![Build Status](https://travis-ci.org/OneOfOne/cmap.svg?branch=master)](https://travis-ci.org/OneOfOne/cmap)
--

CMap (concurrent-map) is a sharded map implementation to support fast concurrent access.

## Install

	go get github.com/OneOfOne/cmap

## FAQ

### Why?
* A simple sync.RWMutex wrapped map is much slower as the concurrency increase.
* Provides several helper functions, Swap(), Update, DeleteAndGet.

### Why not `sync.Map`?
* `sync.Map` is great, I absolute love it if all you need is pure Load/Store, however you can't safely update values in it.
* There was something else major, I can't remember it right now.

## Usage

```go
import (
	"github.com/OneOfOne/cmap"
)

func main() {
	cm := cmap.New() // or cmap.NewString()
	// cm := cmap.NewSize(1 << 8) // the size must always be a power of 2
	cm.Set("key", "value")
	ok := cm.Has("key") == true
	if v, ok := cm.Get("key").(string); ok {
		// do something with v
	}
	cm.Update("key", func(old interface{}) interface{} {
		v, _ := old.(uint64)
		return v + 1
	})
}
```

## Benchmark
```bash
➤ go1.9rc2 test -v -bench=. -benchtime=5s -tags streamrail -benchmem -cpu 8 -short ./ ./stringcmap

goos: linux
goarch: amd64
pkg: github.com/OneOfOne/cmap

BenchmarkCMap/2048-8  	50000000	       147 ns/op	      48 B/op	       3 allocs/op
BenchmarkCMap/4096-8  	50000000	       134 ns/op	      48 B/op	       3 allocs/op
BenchmarkCMap/8192-8  	50000000	       128 ns/op	      48 B/op	       3 allocs/op

# simple map[interface{}]interface{} wrapped with a sync.RWMutex
BenchmarkMutexMap-8   	20000000	       404 ns/op	      32 B/op	       2 allocs/op

# sync.Map
BenchmarkSyncMap-8    	50000000	       141 ns/op	      48 B/op	       3 allocs/op

PASS
ok  	github.com/OneOfOne/cmap	40.197s

goos: linux
goarch: amd64
pkg: github.com/OneOfOne/cmap/stringcmap

# specialized version of CMap, using map[string]interface{} internally
BenchmarkStringCMap/2048-8         	100000000	        61.5 ns/op	      16 B/op	       1 allocs/op
BenchmarkStringCMap/4096-8         	100000000	        58.0 ns/op	      16 B/op	       1 allocs/op
BenchmarkStringCMap/8192-8         	100000000	        51.1 ns/op	      16 B/op	       1 allocs/op

# github.com/streamrail/concurrent-map
BenchmarkStreamrail/2048-8         	100000000	        64.7 ns/op	      16 B/op	       1 allocs/op
BenchmarkStreamrail/4096-8         	100000000	        62.1 ns/op	      16 B/op	       1 allocs/op
BenchmarkStreamrail/8192-8         	100000000	        61.5 ns/op	      16 B/op	       1 allocs/op

PASS
ok  	github.com/OneOfOne/cmap/stringcmap	36.413s
```

## License

Apache v2.0 (see [LICENSE](https://github.com/OneOfOne/cmap/blob/master/LICENSE) file).

Copyright 2016-2017 Ahmed <[OneOfOne](https://github.com/OneOfOne/)> W.

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

		http://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
