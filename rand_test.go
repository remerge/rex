package rex

import (
	coreRand "math/rand"
	"testing"
	"time"

	"github.com/remerge/rex/rand"
)

func BenchmarkCoreRandGlobal(b *testing.B) {
	var acc int64
	for i := 0; i < b.N; i++ {
		acc += coreRand.Int63()
	}
	_ = acc
}

func BenchmarkCoreRand(b *testing.B) {
	generator := coreRand.New(coreRand.NewSource(time.Now().UnixNano()))
	var acc int64
	for i := 0; i < b.N; i++ {
		acc += generator.Int63()
	}
	_ = acc
}

func BenchmarkCoreRandEach(b *testing.B) {
	var acc int64
	for i := 0; i < b.N; i++ {
		generator := coreRand.New(coreRand.NewSource(time.Now().UnixNano()))
		acc += generator.Int63()
	}
	_ = acc
}

func BenchmarkRexRandGlobal(b *testing.B) {
	var acc int64
	for i := 0; i < b.N; i++ {
		acc += rand.Int63()
	}
	_ = acc
}

func BenchmarkRexRand(b *testing.B) {
	generator := rand.NewXorRand(uint64(time.Now().UnixNano()), uint64(time.Now().UnixNano()))
	var acc int64
	for i := 0; i < b.N; i++ {
		acc += generator.Int63()
	}
	_ = acc
}

func BenchmarkRexRandEach(b *testing.B) {
	var acc int64
	for i := 0; i < b.N; i++ {
		generator := rand.NewXorRand(uint64(time.Now().UnixNano()), uint64(time.Now().UnixNano()))
		acc += generator.Int63()
	}
	_ = acc
}

func BenchmarkCoreRandGlobalParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		var acc int64
		for pb.Next() {
			acc += coreRand.Int63()
		}
		_ = acc
	})
}

func BenchmarkCoreRandParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		generator := coreRand.New(coreRand.NewSource(time.Now().UnixNano()))
		var acc int64
		for pb.Next() {
			acc += generator.Int63()
		}
		_ = acc
	})
}

func BenchmarkCoreRandEachParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		var acc int64
		for pb.Next() {
			generator := coreRand.New(coreRand.NewSource(time.Now().UnixNano()))
			acc += generator.Int63()
		}
		_ = acc
	})
}

func BenchmarkRexRandGlobalParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		var acc int64
		for pb.Next() {
			acc += rand.Int63()
		}
		_ = acc
	})
}

func BenchmarkRexRandParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		generator := rand.NewXorRand(uint64(time.Now().UnixNano()), uint64(time.Now().UnixNano()))
		var acc int64
		for pb.Next() {
			acc += generator.Int63()
		}
		_ = acc
	})
}

func BenchmarkRexRandEachParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		var acc int64
		for pb.Next() {
			generator := rand.NewXorRand(uint64(time.Now().UnixNano()), uint64(time.Now().UnixNano()))
			acc += generator.Int63()
		}
		_ = acc
	})
}
