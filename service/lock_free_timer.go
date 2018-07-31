package service

import (
	"sync"
	"sync/atomic"
	"time"

	metrics "github.com/rcrowley/go-metrics"
	"github.com/remerge/rand"
)

func GetOrRegisterLockFreeTimer(name string, r metrics.Registry) metrics.Timer {
	if nil == r {
		r = metrics.DefaultRegistry
	}
	return r.GetOrRegister(name, NewLockFreeTimer).(metrics.Timer)
}

func NewLockFreeTimer() metrics.Timer {
	return &LockFreeTimer{
		counter:   metrics.NewCounter(),
		histogram: metrics.NewHistogram(NewLockFreeSample(1028)),
	}
}

type LockFreeTimer struct {
	counter   metrics.Counter
	histogram metrics.Histogram
}

func (t *LockFreeTimer) Stop() {}

func (t *LockFreeTimer) Count() int64 {
	return t.counter.Count()
}

func (t *LockFreeTimer) Max() int64 {
	return t.histogram.Max()
}

func (t *LockFreeTimer) Mean() float64 {
	return t.histogram.Mean()
}

func (t *LockFreeTimer) Min() int64 {
	return t.histogram.Min()
}

func (t *LockFreeTimer) Percentile(p float64) float64 {
	return t.histogram.Percentile(p)
}

func (t *LockFreeTimer) Percentiles(ps []float64) []float64 {
	return t.histogram.Percentiles(ps)
}

func (t *LockFreeTimer) Rate1() float64 {
	return 0.0
}

func (t *LockFreeTimer) Rate5() float64 {
	return 0.0
}

func (t *LockFreeTimer) Rate15() float64 {
	return 0.0
}

func (t *LockFreeTimer) RateMean() float64 {
	return 0.0
}

func (t *LockFreeTimer) Snapshot() metrics.Timer {
	return &LockFreeTimerSnapshot{
		counter:   t.counter.Snapshot().(metrics.CounterSnapshot),
		histogram: t.histogram.Snapshot().(*metrics.HistogramSnapshot),
	}
}

func (t *LockFreeTimer) StdDev() float64 {
	return t.histogram.StdDev()
}

func (t *LockFreeTimer) Sum() int64 {
	return t.histogram.Sum()
}

func (t *LockFreeTimer) Time(f func()) {
	ts := time.Now()
	f()
	t.Update(time.Since(ts))
}

func (t *LockFreeTimer) Update(d time.Duration) {
	t.counter.Inc(1)
	t.histogram.Update(int64(d))
}

func (t *LockFreeTimer) UpdateSince(ts time.Time) {
	t.Update(time.Since(ts))
}

func (t *LockFreeTimer) Variance() float64 {
	return t.histogram.Variance()
}

type LockFreeTimerSnapshot struct {
	counter   metrics.CounterSnapshot
	histogram *metrics.HistogramSnapshot
}

func (t *LockFreeTimerSnapshot) Stop() {}

func (t *LockFreeTimerSnapshot) Count() int64 { return t.counter.Count() }

func (t *LockFreeTimerSnapshot) Max() int64 { return t.histogram.Max() }

func (t *LockFreeTimerSnapshot) Mean() float64 { return t.histogram.Mean() }

func (t *LockFreeTimerSnapshot) Min() int64 { return t.histogram.Min() }

func (t *LockFreeTimerSnapshot) Percentile(p float64) float64 {
	return t.histogram.Percentile(p)
}

func (t *LockFreeTimerSnapshot) Percentiles(ps []float64) []float64 {
	return t.histogram.Percentiles(ps)
}

func (t *LockFreeTimerSnapshot) Rate1() float64 { return 0.0 }

func (t *LockFreeTimerSnapshot) Rate5() float64 { return 0.0 }

func (t *LockFreeTimerSnapshot) Rate15() float64 { return 0.0 }

func (t *LockFreeTimerSnapshot) RateMean() float64 { return 0.0 }

func (t *LockFreeTimerSnapshot) Snapshot() metrics.Timer { return t }

func (t *LockFreeTimerSnapshot) StdDev() float64 { return t.histogram.StdDev() }

func (t *LockFreeTimerSnapshot) Sum() int64 { return t.histogram.Sum() }

func (*LockFreeTimerSnapshot) Time(func()) {
	panic("Time called on a LockFreeTimerSnapshot")
}

func (*LockFreeTimerSnapshot) Update(time.Duration) {
	panic("Update called on a LockFreeTimerSnapshot")
}

func (*LockFreeTimerSnapshot) UpdateSince(time.Time) {
	panic("UpdateSince called on a LockFreeTimerSnapshot")
}

func (t *LockFreeTimerSnapshot) Variance() float64 { return t.histogram.Variance() }

type LockFreeSample struct {
	count  int64
	mutex  sync.Mutex
	values []int64
}

func NewLockFreeSample(reservoirSize int) metrics.Sample {
	return &LockFreeSample{
		values: make([]int64, reservoirSize),
	}
}

func (s *LockFreeSample) Clear() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.count = 0
	s.values = make([]int64, cap(s.values))
}

func (s *LockFreeSample) Count() int64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.count
}

func (s *LockFreeSample) Max() int64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return metrics.SampleMax(s.values)
}

func (s *LockFreeSample) Mean() float64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return metrics.SampleMean(s.values)
}

func (s *LockFreeSample) Min() int64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return metrics.SampleMin(s.values)
}

func (s *LockFreeSample) Percentile(p float64) float64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return metrics.SamplePercentile(s.values, p)
}

func (s *LockFreeSample) Percentiles(ps []float64) []float64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return metrics.SamplePercentiles(s.values, ps)
}

func (s *LockFreeSample) Size() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return len(s.values)
}

func (s *LockFreeSample) Snapshot() metrics.Sample {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	count := atomic.SwapInt64(&s.count, 0)
	values := make([]int64, min(int(count), len(s.values)))
	copy(values, s.values)
	return metrics.NewSampleSnapshot(count, values)
}

func (s *LockFreeSample) StdDev() float64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return metrics.SampleStdDev(s.values)
}

func (s *LockFreeSample) Sum() int64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return metrics.SampleSum(s.values)
}

func (s *LockFreeSample) Update(v int64) {
	// we accept a data race here to reduce lock
	// contention and to increase performance
	count := atomic.AddInt64(&s.count, 1)
	if int(count) <= len(s.values) {
		s.values[count-1] = v
	} else {
		r := rand.Int63n(count)
		if int(r) < len(s.values) {
			s.values[r] = v
		}
	}
}

func (s *LockFreeSample) Values() []int64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	values := make([]int64, len(s.values))
	copy(values, s.values)
	return values
}

func (s *LockFreeSample) Variance() float64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return metrics.SampleVariance(s.values)
}

func min(s, v int) int {
	if s <= v {
		return s
	}
	return v
}
