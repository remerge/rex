package rex

import (
	"math"
	"sync/atomic"
	"time"

	"github.com/remerge/rand"
)

type BackoffCallback struct {
	n uint64
	c uint64
}

func (b *BackoffCallback) Step(f func()) {
	cN := atomic.LoadUint64(&b.n)
	cC := atomic.LoadUint64(&b.c)
	if cN == uint64(((math.Exp2(float64(cC))-1)/2)+0.5) {
		atomic.CompareAndSwapUint64(&b.c, cC, cC+1)
		f()
	}
	atomic.AddUint64(&b.n, 1)
}

func (b *BackoffCallback) Reset() {
	atomic.StoreUint64(&b.c, 0)
	atomic.StoreUint64(&b.n, 0)
}

// BackoffDuration is a time.Duration counter. It starts at Min.
// After every call to Duration() it is multiplied by Factor.
// It is capped at Max. It returns to Min on every call to Reset().
// Used in conjunction with the time package.
type BackoffDuration struct {
	// Factor is the multiplying factor for each increment step
	attempts, Factor float64
	// Jitter eases contention by randomizing backoff steps
	Jitter bool
	// Min and Max are the minimum and maximum values of the counter
	Min, Max time.Duration
}

// Duration returns the current value of the counter and then
// multiplies it Factor
func (b *BackoffDuration) Duration() time.Duration {
	// Zero-values are nonsensical, so we use
	// them to apply defaults
	if b.Min == 0 {
		b.Min = 100 * time.Millisecond
	}
	if b.Max == 0 {
		b.Max = 10 * time.Second
	}
	if b.Factor == 0 {
		b.Factor = 2
	}
	// calculate this duration
	dur := float64(b.Min) * math.Pow(b.Factor, b.attempts)
	if b.Jitter == true {
		dur = rand.Float64()*(dur-float64(b.Min)) + float64(b.Min)
	}
	// cap!
	if dur > float64(b.Max) {
		return b.Max
	}
	// bump attempts count
	b.attempts++
	// return as a time.Duration
	return time.Duration(dur)
}

// Resets the current value of the counter back to Min
func (b *BackoffDuration) Reset() {
	b.attempts = 0
}
