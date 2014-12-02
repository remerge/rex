package rex

import (
	"errors"
	"sync"
	"time"

	"github.com/rcrowley/go-metrics"
)

type Derive interface {
	Count() int64
	Mark(int64)
	Rate1() float64
	Rate5() float64
	Rate15() float64
	RateMean() float64
	Snapshot() metrics.Meter
}

func GetOrRegisterDerive(name string, r metrics.Registry) Derive {
	if nil == r {
		r = metrics.DefaultRegistry
	}
	return r.GetOrRegister(name, NewDerive).(Derive)
}

func NewDerive(name string) Derive {
	if metrics.UseNilMetrics {
		return NilDerive{}
	}
	m := newStandardDerive()
	arbiter.Lock()
	defer arbiter.Unlock()
	arbiter.derives = append(arbiter.derives, m)
	if !arbiter.started {
		arbiter.started = true
		go arbiter.tick()
	}
	metrics.DefaultRegistry.Register(name, (metrics.Meter)(m))
	return m
}

// DeriveSnapshot is a read-only copy of another Derive.
type DeriveSnapshot struct {
	count                          int64
	rate1, rate5, rate15, rateMean float64
}

// Count returns the count of events at the time the snapshot was taken.
func (m *DeriveSnapshot) Count() int64 { return m.count }

func (*DeriveSnapshot) Mark(n int64) {
	MayPanic(errors.New("Mark called on a DeriveSnapshot"))
}

// Rate1 returns the one-minute moving average rate of events per second at the
// time the snapshot was taken.
func (m *DeriveSnapshot) Rate1() float64 { return m.rate1 }

// Rate5 returns the five-minute moving average rate of events per second at
// the time the snapshot was taken.
func (m *DeriveSnapshot) Rate5() float64 { return m.rate5 }

// Rate15 returns the fifteen-minute moving average rate of events per second
// at the time the snapshot was taken.
func (m *DeriveSnapshot) Rate15() float64 { return m.rate15 }

// RateMean returns the derive's mean rate of events per second at the time the
// snapshot was taken.
func (m *DeriveSnapshot) RateMean() float64 { return m.rateMean }

// Snapshot returns the snapshot.
func (m *DeriveSnapshot) Snapshot() metrics.Meter { return m }

// NilDerive is a no-op Derive.
type NilDerive struct{}

// Count is a no-op.
func (NilDerive) Count() int64 { return 0 }

// Mark is a no-op.
func (NilDerive) Mark(n int64) {}

// Rate1 is a no-op.
func (NilDerive) Rate1() float64 { return 0.0 }

// Rate5 is a no-op.
func (NilDerive) Rate5() float64 { return 0.0 }

// Rate15is a no-op.
func (NilDerive) Rate15() float64 { return 0.0 }

// RateMean is a no-op.
func (NilDerive) RateMean() float64 { return 0.0 }

// Snapshot is a no-op.
func (NilDerive) Snapshot() metrics.Meter { return NilDerive{} }

// StandardDerive is the standard implementation of a Derive.
type StandardDerive struct {
	lock        sync.RWMutex
	snapshot    *DeriveSnapshot
	a1, a5, a15 metrics.EWMA
	startTime   time.Time
}

func newStandardDerive() *StandardDerive {
	return &StandardDerive{
		snapshot:  &DeriveSnapshot{},
		a1:        metrics.NewEWMA1(),
		a5:        metrics.NewEWMA5(),
		a15:       metrics.NewEWMA15(),
		startTime: time.Now(),
	}
}

// Count returns the number of events recorded.
func (m *StandardDerive) Count() int64 {
	m.lock.RLock()
	count := m.snapshot.count
	m.lock.RUnlock()
	return count
}

// Mark records the occurance of n events derived from a counter.
func (m *StandardDerive) Mark(counter int64) {
	m.lock.Lock()
	defer m.lock.Unlock()
	var n int64
	if m.snapshot.count == 0 {
		m.snapshot.count = counter
		n = 0
	} else if counter < m.snapshot.count {
		n = counter
	} else {
		n = counter - m.snapshot.count
	}
	m.snapshot.count += n
	m.a1.Update(n)
	m.a5.Update(n)
	m.a15.Update(n)
	m.updateSnapshot()
}

// Rate1 returns the one-minute moving average rate of events per second.
func (m *StandardDerive) Rate1() float64 {
	m.lock.RLock()
	rate1 := m.snapshot.rate1
	m.lock.RUnlock()
	return rate1
}

// Rate5 returns the five-minute moving average rate of events per second.
func (m *StandardDerive) Rate5() float64 {
	m.lock.RLock()
	rate5 := m.snapshot.rate5
	m.lock.RUnlock()
	return rate5
}

// Rate15 returns the fifteen-minute moving average rate of events per second.
func (m *StandardDerive) Rate15() float64 {
	m.lock.RLock()
	rate15 := m.snapshot.rate15
	m.lock.RUnlock()
	return rate15
}

// RateMean returns the derive's mean rate of events per second.
func (m *StandardDerive) RateMean() float64 {
	m.lock.RLock()
	rateMean := m.snapshot.rateMean
	m.lock.RUnlock()
	return rateMean
}

// Snapshot returns a read-only copy of the derive.
func (m *StandardDerive) Snapshot() metrics.Meter {
	m.lock.RLock()
	snapshot := *m.snapshot
	m.lock.RUnlock()
	return &snapshot
}

func (m *StandardDerive) updateSnapshot() {
	// should run with write lock held on m.lock
	snapshot := m.snapshot
	snapshot.rate1 = m.a1.Rate()
	snapshot.rate5 = m.a5.Rate()
	snapshot.rate15 = m.a15.Rate()
	snapshot.rateMean = float64(snapshot.count) / time.Since(m.startTime).Seconds()
}

func (m *StandardDerive) tick() {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.a1.Tick()
	m.a5.Tick()
	m.a15.Tick()
	m.updateSnapshot()
}

type deriveArbiter struct {
	sync.RWMutex
	started bool
	derives []*StandardDerive
	ticker  *time.Ticker
}

var arbiter = deriveArbiter{ticker: time.NewTicker(5e9)}

// Ticks derives on the scheduled interval
func (ma *deriveArbiter) tick() {
	for {
		select {
		case <-ma.ticker.C:
			ma.tickDerives()
		}
	}
}

func (ma *deriveArbiter) tickDerives() {
	ma.RLock()
	defer ma.RUnlock()
	for _, derive := range ma.derives {
		derive.tick()
	}
}
