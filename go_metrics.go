package rex

import (
	"runtime"

	"github.com/heroku/instruments"
	"github.com/heroku/instruments/reporter"
)

var goMetrics *GoMetrics

type GoMetrics struct {
	NumGoRoutines *instruments.Gauge
	Alloc         *instruments.Gauge
	TotalAlloc    *instruments.Gauge
	HeapInUse     *instruments.Gauge
	HeapObjects   *instruments.Gauge
	StackInUse    *instruments.Gauge
	NumGC         *instruments.Gauge
	GCPause       *instruments.Gauge
	Mallocs       *instruments.Gauge
	Frees         *instruments.Gauge
}

func InitGoRuntimeMetrics() {
	if goMetrics == nil {
		goMetrics = newGoMetrics()
	}
}

func newGoMetrics() *GoMetrics {
	return &GoMetrics{
		NumGoRoutines: reporter.NewRegisteredGauge("go.num_go_routines", 0),
		Alloc:         reporter.NewRegisteredGauge("go.alloc", 0),
		TotalAlloc:    reporter.NewRegisteredGauge("go.total_alloc", 0),
		HeapInUse:     reporter.NewRegisteredGauge("go.heap_in_use", 0),
		HeapObjects:   reporter.NewRegisteredGauge("go.heap_objects", 0),
		StackInUse:    reporter.NewRegisteredGauge("go.stack_in_use", 0),
		NumGC:         reporter.NewRegisteredGauge("go.num_gc", 0),
		GCPause:       reporter.NewRegisteredGauge("go.gc_pause", 0),
		Mallocs:       reporter.NewRegisteredGauge("go.mallocs", 0),
		Frees:         reporter.NewRegisteredGauge("go.frees", 0),
	}
}

func (self *GoMetrics) update() {
	self.NumGoRoutines.Update(int64(runtime.NumGoroutine()))
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	self.Alloc.Update(int64(m.Alloc))
	self.TotalAlloc.Update(int64(m.TotalAlloc))
	self.HeapInUse.Update(int64(m.HeapInuse))
	self.HeapObjects.Update(int64(m.HeapObjects))
	self.StackInUse.Update(int64(m.StackInuse))
	self.NumGC.Update(int64(m.NumGC))
	self.GCPause.Update(int64(m.PauseNs[(m.NumGC+255)%256]))
	self.Mallocs.Update(int64(m.Mallocs))
	self.Frees.Update(int64(m.Frees))
}
