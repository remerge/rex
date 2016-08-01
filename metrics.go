//go:generate easyjson -all metrics.go
package rex

import (
	"fmt"
	"time"

	"github.com/heroku/instruments"
	"github.com/heroku/instruments/reporter"
	"github.com/juju/loggo"
)

type MetricEvent struct {
	Event
	Name   string  `json:"name,omitempty"`
	Type   string  `json:"type,omitempty"`
	Value  int64   `json:"value,omitempty"`
	M1     int64   `json:"m1,omitempty"`
	Min    int64   `json:"min,omitempty"`
	Max    int64   `json:"max,omitempty"`
	Mean   float64 `json:"mean,omitempty"`
	Stddev float64 `json:"stddev,omitempty"`
	P50    int64   `json:"p50,omitempty"`
	P75    int64   `json:"p75,omitempty"`
	P95    int64   `json:"p95,omitempty"`
	P99    int64   `json:"p99,omitempty"`
	P999   int64   `json:"p999,omitempty"`
}

func (e *MetricEvent) Base() *Event {
	return &e.Event
}

func UpdateTimer(timer *instruments.Timer, start time.Time) {
	timer.Update(time.Since(start))
}

type MetricsTicker struct {
	cb        func(tracker Tracker)
	goMetrics *GoMetrics
	tracker   Tracker
	ticker    *time.Ticker
	quit      chan bool
	done      chan bool
}

func NewMetricsTicker(t Tracker) *MetricsTicker {
	return &MetricsTicker{
		goMetrics: NewGoMetrics(),
		ticker:    time.NewTicker(time.Minute),
		quit:      make(chan bool, 1),
		done:      make(chan bool, 1),
		tracker:   t,
	}
}

func (ticker *MetricsTicker) SetCallback(cb func(tracker Tracker)) {
	ticker.cb = cb
}

func (self *MetricsTicker) Start() {
	for {
		select {
		case <-self.ticker.C:
			self.goMetrics.Update()
			self.Track()
		case <-self.quit:
			self.ticker.Stop()
			self.Track()
			close(self.quit)
			close(self.done)
			return
		}
	}
}

func (self *MetricsTicker) Stop() {
	loggo.GetLogger("rex.metrics").Infof("stopping metrics ticker")
	self.quit <- true
	<-self.done
	loggo.GetLogger("rex.metrics").Infof("stopped metrics ticker")
}

func (self *MetricsTicker) Track() {
	for name, i := range reporter.DefaultRegistry.Instruments() {
		event := &MetricEvent{
			Name: name,
		}

		switch instrument := i.(type) {
		case *instruments.Gauge:
			event.Type = "gauge"
			event.Value = instrument.Snapshot()
		case *instruments.Counter:
			event.Type = "counter"
			event.Value = instrument.Snapshot()
		case *instruments.Rate:
			event.Type = "meter" // backwards compat
			event.Value = instrument.Snapshot()
			event.M1 = event.Value // backwards compat
		case *instruments.Derive:
			event.Type = "derive"
			event.Value = instrument.Snapshot()
			event.M1 = event.Value // backwards compat
		case *instruments.Reservoir:
			s := instrument.Snapshot()
			event.Type = "histogram" // backwards compat
			event.Min = instruments.Min(s)
			event.Max = instruments.Max(s)
			event.Mean = instruments.Mean(s)
			event.Stddev = instruments.StandardDeviation(s)
			event.P50 = instruments.Quantile(s, 0.50)
			event.P75 = instruments.Quantile(s, 0.75)
			event.P95 = instruments.Quantile(s, 0.95)
			event.P99 = instruments.Quantile(s, 0.99)
			event.P999 = instruments.Quantile(s, 0.999)
		case *instruments.Timer:
			s := instrument.Snapshot()
			event.Type = "timer"
			event.Value = instrument.Rate.Snapshot()
			event.M1 = event.Value // backwards compat
			event.Min = instruments.Min(s)
			event.Max = instruments.Max(s)
			event.Mean = instruments.Mean(s)
			event.Stddev = instruments.StandardDeviation(s)
			event.P50 = instruments.Quantile(s, 0.50)
			event.P75 = instruments.Quantile(s, 0.75)
			event.P95 = instruments.Quantile(s, 0.95)
			event.P99 = instruments.Quantile(s, 0.99)
			event.P999 = instruments.Quantile(s, 0.999)
		default:
			panic(fmt.Sprintf("unknown instrument %#v", i))
		}
		self.tracker.SafeEvent("metrics", event, true)
	}

	if self.cb != nil {
		self.cb(self.tracker)
	}
}
