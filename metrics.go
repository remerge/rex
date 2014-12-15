package rex

import (
	"time"

	"github.com/juju/loggo"
	"github.com/rcrowley/go-metrics"
)

func NewGauge(name string) metrics.Gauge {
	return metrics.NewRegisteredGauge(name, metrics.DefaultRegistry)
}

func NewGaugeFloat64(name string) metrics.GaugeFloat64 {
	return metrics.NewRegisteredGaugeFloat64(name, metrics.DefaultRegistry)
}

func NewCounter(name string) metrics.Counter {
	return metrics.NewRegisteredCounter(name, metrics.DefaultRegistry)
}

func NewHistogram(name string) metrics.Histogram {
	return metrics.NewRegisteredHistogram(name, metrics.DefaultRegistry, metrics.NewExpDecaySample(1028, 0.015))
}

func NewMeter(name string) metrics.Meter {
	return metrics.NewRegisteredMeter(name, metrics.DefaultRegistry)
}

func NewTimer(name string) metrics.Timer {
	return metrics.NewRegisteredTimer(name, metrics.DefaultRegistry)
}

func UpdateTimer(timer metrics.Timer, start time.Time) {
	timer.Update(time.Since(start))
}

type MetricsTicker struct {
	tracker Tracker
	ticker  *time.Ticker
	quit    chan bool
	done    chan bool
}

func NewMetricsTicker(t Tracker) *MetricsTicker {
	return &MetricsTicker{
		ticker:  time.NewTicker(10 * time.Second),
		quit:    make(chan bool, 1),
		done:    make(chan bool, 1),
		tracker: t,
	}
}

func (self *MetricsTicker) Start() {
	for {
		select {
		case <-self.ticker.C:
			self.Track()
		case <-self.quit:
			self.ticker.Stop()
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
	metrics.DefaultRegistry.Each(func(name string, i interface{}) {

		event := make(map[string]interface{})
		event["name"] = name

		switch metric := i.(type) {
		case metrics.Counter:
			event["type"] = "counter"
			event["count"] = metric.Count()
		case metrics.Gauge:
			event["type"] = "gauge"
			event["value"] = metric.Value()
		case metrics.GaugeFloat64:
			event["type"] = "gauge"
			event["value"] = metric.Value()
		case metrics.Histogram:
			event["type"] = "histogram"
			h := metric.Snapshot()
			ps := h.Percentiles([]float64{0.5, 0.75, 0.95, 0.99, 0.999})
			event["count"] = h.Count()
			event["min"] = h.Min()
			event["max"] = h.Max()
			event["mean"] = h.Mean()
			event["stddev"] = h.StdDev()
			event["p50"] = ps[0]
			event["p75"] = ps[1]
			event["p95"] = ps[2]
			event["p99"] = ps[3]
			event["p999"] = ps[4]
		case metrics.Meter:
			event["type"] = "meter"
			m := metric.Snapshot()
			event["count"] = m.Count()
			event["m1"] = m.Rate1()
			event["m5"] = m.Rate5()
			event["m15"] = m.Rate15()
		case Derive:
			event["type"] = "meter"
			m := metric.Snapshot()
			event["count"] = m.Count()
			event["m1"] = m.Rate1()
			event["m5"] = m.Rate5()
			event["m15"] = m.Rate15()
		case metrics.Timer:
			event["type"] = "timer"
			t := metric.Snapshot()
			ps := t.Percentiles([]float64{0.5, 0.75, 0.95, 0.99, 0.999})
			event["count"] = t.Count()
			event["min"] = time.Duration(t.Min()) / time.Millisecond
			event["max"] = time.Duration(t.Max()) / time.Millisecond
			event["mean"] = time.Duration(t.Mean()) / time.Millisecond
			event["stddev"] = time.Duration(t.StdDev()) / time.Millisecond
			event["median"] = time.Duration(ps[0]) / time.Millisecond
			event["p75"] = time.Duration(ps[1]) / time.Millisecond
			event["p95"] = time.Duration(ps[2]) / time.Millisecond
			event["p99"] = time.Duration(ps[3]) / time.Millisecond
			event["p999"] = time.Duration(ps[4]) / time.Millisecond
			event["m1"] = t.Rate1()
			event["m5"] = t.Rate5()
			event["m15"] = t.Rate15()
		}
		self.tracker.EventMap("metrics", event, true)
	})
}
