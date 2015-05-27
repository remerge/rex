package rex

import (
	"fmt"
	"time"

	"github.com/heroku/instruments"
	"github.com/heroku/instruments/reporter"
	"github.com/juju/loggo"
)

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
	for name, i := range reporter.DefaultRegistry.Instruments() {
		event := make(map[string]interface{})
		event["name"] = name

		switch instrument := i.(type) {
		case *instruments.Gauge:
			event["type"] = "gauge"
			event["value"] = instrument.Snapshot()
		case *instruments.Counter:
			event["type"] = "counter"
			event["value"] = instrument.Snapshot()
		case *instruments.Rate:
			event["type"] = "meter" // backwards compat
			event["value"] = instrument.Snapshot()
			event["m1"] = event["value"] // backwards compat
		case *instruments.Derive:
			event["type"] = "derive"
			event["value"] = instrument.Snapshot()
		case *instruments.Reservoir:
			s := instrument.Snapshot()
			event["type"] = "histogram" // backwards compat
			event["min"] = instruments.Min(s)
			event["max"] = instruments.Max(s)
			event["mean"] = instruments.Mean(s)
			event["stddev"] = instruments.StandardDeviation(s)
			event["p50"] = instruments.Quantile(s, 0.50)
			event["p75"] = instruments.Quantile(s, 0.75)
			event["p95"] = instruments.Quantile(s, 0.95)
			event["p99"] = instruments.Quantile(s, 0.99)
			event["p999"] = instruments.Quantile(s, 0.999)
		case *instruments.Timer:
			s := instrument.Snapshot()
			event["type"] = "timer"
			event["value"] = instrument.Rate.Snapshot()
			event["m1"] = event["value"] // backwards compat
			event["min"] = instruments.Min(s)
			event["max"] = instruments.Max(s)
			event["mean"] = instruments.Mean(s)
			event["stddev"] = instruments.StandardDeviation(s)
			event["p50"] = instruments.Quantile(s, 0.50)
			event["p75"] = instruments.Quantile(s, 0.75)
			event["p95"] = instruments.Quantile(s, 0.95)
			event["p99"] = instruments.Quantile(s, 0.99)
			event["p999"] = instruments.Quantile(s, 0.999)
		default:
			panic(fmt.Sprintf("unknown instrument %#v", i))
		}
		self.tracker.SafeEventMap("metrics", event, true)
	}
}
