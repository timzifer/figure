package three

import "sync"

// Everything sized by the data comes from a pool and is handed back, which is
// what makes a scene redrawn on every pointer move affordable: a frame over a
// surface of a hundred thousand quads costs the same handful of allocations as
// one over a hundred. ADR 0057 decided this before any of it was written, and
// the allocation gate holds it.
var scratchPool = sync.Pool{New: func() any { return new(Sink) }}

func acquire() *Sink {
	s := scratchPool.Get().(*Sink)
	s.reset()
	return s
}

func release(s *Sink) { scratchPool.Put(s) }

// grow returns a slice of length n backed by buf's array where it fits, which
// is the shape every buffer in this package is reused through. It is geom's
// helper, and it is copied rather than exported from there because a pool
// helper is not an API.
func grow[T any](buf []T, n int) []T {
	if cap(buf) >= n {
		return buf[:n]
	}
	return make([]T, n)
}
