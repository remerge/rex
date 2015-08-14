package rex

import (
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/heroku/instruments"
	"github.com/heroku/instruments/reporter"
)

var goMetrics *GoMetrics

type MemMetrics struct {
	// General statistics.
	Alloc      *instruments.Gauge  // bytes allocated and still in use
	TotalAlloc *instruments.Gauge  // bytes allocated (even if freed)
	Sys        *instruments.Gauge  // bytes obtained from system (sum of XxxSys below)
	Lookups    *instruments.Derive // number of pointer lookups
	Mallocs    *instruments.Derive // number of mallocs
	Frees      *instruments.Derive // number of frees

	// Main allocation heap statistics.
	HeapAlloc    *instruments.Gauge // bytes allocated and still in use
	HeapSys      *instruments.Gauge // bytes obtained from system
	HeapIdle     *instruments.Gauge // bytes in idle spans
	HeapInuse    *instruments.Gauge // bytes in non-idle span
	HeapReleased *instruments.Gauge // bytes released to the OS
	HeapObjects  *instruments.Gauge // total number of allocated objects

	// Low-level fixed-size structure allocator statistics.
	//	Inuse is bytes used now.
	//	Sys is bytes obtained from system.
	StackInuse  *instruments.Gauge // bytes used by stack allocator
	StackSys    *instruments.Gauge
	MSpanInuse  *instruments.Gauge // mspan structures
	MSpanSys    *instruments.Gauge
	MCacheInuse *instruments.Gauge // mcache structures
	MCacheSys   *instruments.Gauge
	BuckHashSys *instruments.Gauge // profiling bucket hash table
	GCSys       *instruments.Gauge // GC metadata
	OtherSys    *instruments.Gauge // other system allocations

	// Garbage collector statistics.
	Pauses *instruments.Reservoir
	NumGC  uint32
}

type GoMetrics struct {
	sync.Mutex
	GoRoutines *instruments.Gauge
	CgoCalls   *instruments.Derive
	Mem        MemMetrics
}

func NewGoMetrics() *GoMetrics {
	return &GoMetrics{
		GoRoutines: reporter.NewRegisteredGauge("go.go_routines", 0),
		CgoCalls:   reporter.NewRegisteredDerive("go.cgo_calls", 0),
		Mem: MemMetrics{
			Alloc:        reporter.NewRegisteredGauge("go.mem.alloc", 0),
			TotalAlloc:   reporter.NewRegisteredGauge("go.mem.total_alloc", 0),
			Sys:          reporter.NewRegisteredGauge("go.mem.sys", 0),
			Lookups:      reporter.NewRegisteredDerive("go.mem.lookups", 0),
			Mallocs:      reporter.NewRegisteredDerive("go.mem.mallocs", 0),
			Frees:        reporter.NewRegisteredDerive("go.mem.frees", 0),
			HeapAlloc:    reporter.NewRegisteredGauge("go.mem.heap_alloc", 0),
			HeapSys:      reporter.NewRegisteredGauge("go.mem.heap_sys", 0),
			HeapIdle:     reporter.NewRegisteredGauge("go.mem.heap_idle", 0),
			HeapInuse:    reporter.NewRegisteredGauge("go.mem.heap_inuse", 0),
			HeapReleased: reporter.NewRegisteredGauge("go.mem.heap_released", 0),
			HeapObjects:  reporter.NewRegisteredGauge("go.mem.heap_objects", 0),
			StackInuse:   reporter.NewRegisteredGauge("go.mem.stack_inuse", 0),
			StackSys:     reporter.NewRegisteredGauge("go.mem.stack_sys", 0),
			MSpanInuse:   reporter.NewRegisteredGauge("go.mem.mspan_inuse", 0),
			MSpanSys:     reporter.NewRegisteredGauge("go.mem.mspan_sys", 0),
			MCacheInuse:  reporter.NewRegisteredGauge("go.mem.mcache_inuse", 0),
			MCacheSys:    reporter.NewRegisteredGauge("go.mem.mcache_sys", 0),
			BuckHashSys:  reporter.NewRegisteredGauge("go.mem.buck_hash_sys", 0),
			OtherSys:     reporter.NewRegisteredGauge("go.mem.other_sys", 0),
			Pauses:       reporter.NewRegisteredReservoir("go.gc.pauses", 0),
		},
	}
}

func (self *GoMetrics) Update() {
	self.Lock()
	defer self.Unlock()

	self.GoRoutines.Update(int64(runtime.NumGoroutine()))
	self.CgoCalls.Update(int64(runtime.NumCgoCall()))

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	self.Mem.Alloc.Update(int64(m.Alloc))
	self.Mem.Sys.Update(int64(m.Sys))
	self.Mem.Lookups.Update(int64(m.Lookups))
	self.Mem.Mallocs.Update(int64(m.Mallocs))
	self.Mem.Frees.Update(int64(m.Frees))
	self.Mem.HeapAlloc.Update(int64(m.HeapAlloc))
	self.Mem.HeapSys.Update(int64(m.HeapSys))
	self.Mem.HeapIdle.Update(int64(m.HeapIdle))
	self.Mem.HeapInuse.Update(int64(m.HeapInuse))
	self.Mem.HeapReleased.Update(int64(m.HeapReleased))
	self.Mem.HeapObjects.Update(int64(m.HeapObjects))
	self.Mem.StackInuse.Update(int64(m.StackInuse))
	self.Mem.StackSys.Update(int64(m.StackSys))
	self.Mem.MSpanInuse.Update(int64(m.MSpanInuse))
	self.Mem.MSpanSys.Update(int64(m.MSpanSys))
	self.Mem.MCacheInuse.Update(int64(m.MCacheInuse))
	self.Mem.MCacheSys.Update(int64(m.MCacheSys))
	self.Mem.BuckHashSys.Update(int64(m.BuckHashSys))
	self.Mem.OtherSys.Update(int64(m.OtherSys))

	numGC := atomic.SwapUint32(&self.Mem.NumGC, m.NumGC)
	i := numGC % uint32(len(m.PauseNs))
	j := m.NumGC % uint32(len(m.PauseNs))
	if m.NumGC-numGC >= uint32(len(m.PauseNs)) {
		for i = 0; i < uint32(len(m.PauseNs)); i++ {
			self.Mem.Pauses.Update(int64(m.PauseNs[i]))
		}
	} else {
		if i > j {
			for ; i < uint32(len(m.PauseNs)); i++ {
				self.Mem.Pauses.Update(int64(m.PauseNs[i]))
			}
			i = 0
		}
		for ; i < j; i++ {
			self.Mem.Pauses.Update(int64(m.PauseNs[i]))
		}
	}
}
