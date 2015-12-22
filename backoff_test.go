package rex

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestBackoff(t *testing.T) {
	Convey("should backoff properly", t, func() {
		var b BackoffCallback
		correct := []int{0, 1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024, 2048, 4096, 8192}
		var result []int = make([]int, 0)
		for i := 0; i < 10000; i++ {
			b.Step(func() {
				result = append(result, i)
			})
		}
		So(result, ShouldResemble, correct)
	})
}

func Test1(t *testing.T) {

	b := &BackoffDuration{
		Min:    100 * time.Millisecond,
		Max:    10 * time.Second,
		Factor: 2,
	}

	equals(t, b.Duration(), 100*time.Millisecond)
	equals(t, b.Duration(), 200*time.Millisecond)
	equals(t, b.Duration(), 400*time.Millisecond)
	b.Reset()
	equals(t, b.Duration(), 100*time.Millisecond)
}

func Test2(t *testing.T) {

	b := &BackoffDuration{
		Min:    100 * time.Millisecond,
		Max:    10 * time.Second,
		Factor: 1.5,
	}

	equals(t, b.Duration(), 100*time.Millisecond)
	equals(t, b.Duration(), 150*time.Millisecond)
	equals(t, b.Duration(), 225*time.Millisecond)
	b.Reset()
	equals(t, b.Duration(), 100*time.Millisecond)
}

func Test3(t *testing.T) {

	b := &BackoffDuration{
		Min:    100 * time.Nanosecond,
		Max:    10 * time.Second,
		Factor: 1.75,
	}

	equals(t, b.Duration(), 100*time.Nanosecond)
	equals(t, b.Duration(), 175*time.Nanosecond)
	equals(t, b.Duration(), 306*time.Nanosecond)
	b.Reset()
	equals(t, b.Duration(), 100*time.Nanosecond)
}

func TestJitter(t *testing.T) {
	b := &BackoffDuration{
		Min:    100 * time.Millisecond,
		Max:    10 * time.Second,
		Factor: 2,
		Jitter: true,
	}

	equals(t, b.Duration(), 100*time.Millisecond)
	between(t, b.Duration(), 100*time.Millisecond, 200*time.Millisecond)
	between(t, b.Duration(), 100*time.Millisecond, 400*time.Millisecond)
	b.Reset()
	equals(t, b.Duration(), 100*time.Millisecond)
}

func between(t *testing.T, actual, low, high time.Duration) {
	if actual < low {
		t.Fatalf("Got %s, Expecting >= %s", actual, low)
	}
	if actual > high {
		t.Fatalf("Got %s, Expecting <= %s", actual, high)
	}
}

func equals(t *testing.T, d1, d2 time.Duration) {
	if d1 != d2 {
		t.Fatalf("Got %s, Expecting %s", d1, d2)
	}
}
