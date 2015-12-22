package v2

import (
	"fmt"
	"testing"
)

func TestOk(t *testing.T) {

}

type mapKey struct {
	Job   string
	Event string
}

func BenchmarkMapLookup(b *testing.B) {

	m := make(map[mapKey][]byte)

	m[mapKey{"foo", "bar"}] = []byte("hello world")
	m[mapKey{"foo", "baz"}] = []byte("hello world")
	m[mapKey{"", "bar"}] = []byte("hello world")
	m[mapKey{"", "baz"}] = []byte("hello world")
	m[mapKey{"foo1", "bar1"}] = []byte("hello world")
	m[mapKey{"foo2", "bar2"}] = []byte("hello world")
	m[mapKey{"foo3", "bar3"}] = []byte("hello world")
	m[mapKey{"foo4", "bar4"}] = []byte("hello world")
	m[mapKey{"foo5", "bar5"}] = []byte("hello world")

	for i := 0; i < 1000; i++ {
		m[mapKey{fmt.Sprint("foo", i), fmt.Sprint("bar", i)}] = []byte("sup yo")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := mapKey{"foo", "baz"}
		z, ok := m[key]
		if ok {
			zz(z)
		}
	}
}

func zz(b []byte) {

}
