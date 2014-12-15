package rex

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func equal(t *testing.T, act, exp interface{}) {
	if !reflect.DeepEqual(exp, act) {
		_, file, line, _ := runtime.Caller(1)
		t.Logf("\033[31m%s:%d:\n\n\texp: %#v\n\n\tgot: %#v\033[39m\n\n",
			filepath.Base(file), line, exp, act)
		t.FailNow()
	}
}

func nequal(t *testing.T, act, exp interface{}) {
	if reflect.DeepEqual(exp, act) {
		_, file, line, _ := runtime.Caller(1)
		t.Logf("\033[31m%s:%d:\n\n\texp: %#v\n\n\tgot: %#v\033[39m\n\n",
			filepath.Base(file), line, exp, act)
		t.FailNow()
	}
}

func TestDiskQueue(t *testing.T) {
	dqName := "test_disk_queue" + strconv.Itoa(int(time.Now().Unix()))
	dq := NewDiskQueue(dqName, os.TempDir(), 1024, 2500, 2*time.Second)
	nequal(t, dq, nil)
	equal(t, dq.Depth(), int64(0))

	msg := []byte("test")
	err := dq.Put(msg)
	equal(t, err, nil)
	equal(t, dq.Depth(), int64(1))

	msgOut := <-dq.ReadChan()
	equal(t, msgOut, msg)
}

func TestDiskQueueRoll(t *testing.T) {
	dqName := "test_disk_queue_roll" + strconv.Itoa(int(time.Now().Unix()))
	dq := NewDiskQueue(dqName, os.TempDir(), 100, 2500, 2*time.Second)
	nequal(t, dq, nil)
	equal(t, dq.Depth(), int64(0))

	msg := []byte("aaaaaaaaaa")
	for i := 0; i < 10; i++ {
		err := dq.Put(msg)
		equal(t, err, nil)
		equal(t, dq.Depth(), int64(i+1))
	}

	equal(t, dq.writeFileNum, int64(1))
	equal(t, dq.writePos, int64(28))
}

func assertFileNotExist(t *testing.T, fn string) {
	f, err := os.OpenFile(fn, os.O_RDONLY, 0600)
	equal(t, f, (*os.File)(nil))
	equal(t, os.IsNotExist(err), true)
}

func TestDiskQueueEmpty(t *testing.T) {
	dqName := "test_disk_queue_empty" + strconv.Itoa(int(time.Now().Unix()))
	dq := NewDiskQueue(dqName, os.TempDir(), 100, 2500, 2*time.Second)
	nequal(t, dq, nil)
	equal(t, dq.Depth(), int64(0))

	msg := []byte("aaaaaaaaaa")

	for i := 0; i < 100; i++ {
		err := dq.Put(msg)
		equal(t, err, nil)
		equal(t, dq.Depth(), int64(i+1))
	}

	for i := 0; i < 3; i++ {
		<-dq.ReadChan()
	}

	for {
		if dq.Depth() == 97 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	equal(t, dq.Depth(), int64(97))

	numFiles := dq.writeFileNum
	dq.Empty()

	assertFileNotExist(t, dq.metaDataFileName())
	for i := int64(0); i <= numFiles; i++ {
		assertFileNotExist(t, dq.fileName(i))
	}
	equal(t, dq.Depth(), int64(0))
	equal(t, dq.readFileNum, dq.writeFileNum)
	equal(t, dq.readPos, dq.writePos)
	equal(t, dq.nextReadPos, dq.readPos)
	equal(t, dq.nextReadFileNum, dq.readFileNum)

	for i := 0; i < 100; i++ {
		err := dq.Put(msg)
		equal(t, err, nil)
		equal(t, dq.Depth(), int64(i+1))
	}

	for i := 0; i < 100; i++ {
		<-dq.ReadChan()
	}

	for {
		if dq.Depth() == 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	equal(t, dq.Depth(), int64(0))
	equal(t, dq.readFileNum, dq.writeFileNum)
	equal(t, dq.readPos, dq.writePos)
	equal(t, dq.nextReadPos, dq.readPos)
}

func TestDiskQueueCorruption(t *testing.T) {
	dqName := "test_disk_queue_corruption" + strconv.Itoa(int(time.Now().Unix()))
	dq := NewDiskQueue(dqName, os.TempDir(), 1000, 5, 2*time.Second)

	msg := make([]byte, 123)
	for i := 0; i < 25; i++ {
		dq.Put(msg)
	}

	equal(t, dq.Depth(), int64(25))

	// corrupt the 2nd file
	dqFn := dq.fileName(1)
	os.Truncate(dqFn, 500)

	for i := 0; i < 19; i++ {
		equal(t, <-dq.ReadChan(), msg)
	}

	// corrupt the 4th (current) file
	dqFn = dq.fileName(3)
	os.Truncate(dqFn, 100)

	dq.Put(msg)

	equal(t, <-dq.ReadChan(), msg)
}

func TestDiskQueueTorture(t *testing.T) {
	var wg sync.WaitGroup

	dqName := "test_disk_queue_torture" + strconv.Itoa(int(time.Now().Unix()))
	dq := NewDiskQueue(dqName, os.TempDir(), 262144, 2500, 2*time.Second)
	nequal(t, dq, nil)
	equal(t, dq.Depth(), int64(0))

	msg := []byte("aaaaaaaaaabbbbbbbbbbccccccccccddddddddddeeeeeeeeeeffffffffff")

	numWriters := 4
	numReaders := 4
	readExitChan := make(chan int)
	writeExitChan := make(chan int)

	var depth int64
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				time.Sleep(100000 * time.Nanosecond)
				select {
				case <-writeExitChan:
					return
				default:
					err := dq.Put(msg)
					if err == nil {
						atomic.AddInt64(&depth, 1)
					}
				}
			}
		}()
	}

	time.Sleep(1 * time.Second)

	dq.Close()

	t.Logf("closing writeExitChan")
	close(writeExitChan)
	wg.Wait()

	t.Logf("restarting diskqueue")

	dq = NewDiskQueue(dqName, os.TempDir(), 262144, 2500, 2*time.Second)
	nequal(t, dq, nil)
	equal(t, dq.Depth(), depth)

	var read int64
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				time.Sleep(100000 * time.Nanosecond)
				select {
				case m := <-dq.ReadChan():
					equal(t, msg, m)
					atomic.AddInt64(&read, 1)
				case <-readExitChan:
					return
				}
			}
		}()
	}

	t.Logf("waiting for depth 0")
	for {
		if dq.Depth() == 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Logf("closing readExitChan")
	close(readExitChan)
	wg.Wait()

	equal(t, read, depth)

	dq.Close()
}

func BenchmarkDiskQueuePut(b *testing.B) {
	b.StopTimer()
	dqName := "bench_disk_queue_put" + strconv.Itoa(b.N) + strconv.Itoa(int(time.Now().Unix()))
	dq := NewDiskQueue(dqName, os.TempDir(), 1024768*100, 2500, 2*time.Second)
	size := 1024
	b.SetBytes(int64(size))
	data := make([]byte, size)
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		dq.Put(data)
	}
}

func BenchmarkDiskWrite(b *testing.B) {
	b.StopTimer()
	fileName := "bench_disk_queue_put" + strconv.Itoa(b.N) + strconv.Itoa(int(time.Now().Unix()))
	f, _ := os.OpenFile(path.Join(os.TempDir(), fileName), os.O_RDWR|os.O_CREATE, 0600)
	size := 256
	b.SetBytes(int64(size))
	data := make([]byte, size)
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		f.Write(data)
	}
	f.Sync()
}

func BenchmarkDiskWriteBuffered(b *testing.B) {
	b.StopTimer()
	fileName := "bench_disk_queue_put" + strconv.Itoa(b.N) + strconv.Itoa(int(time.Now().Unix()))
	f, _ := os.OpenFile(path.Join(os.TempDir(), fileName), os.O_RDWR|os.O_CREATE, 0600)
	size := 256
	b.SetBytes(int64(size))
	data := make([]byte, size)
	w := bufio.NewWriterSize(f, 1024*4)
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		w.Write(data)
		if i%1024 == 0 {
			w.Flush()
		}
	}
	w.Flush()
	f.Sync()
}

// this benchmark should be run via:
//    $ go test -test.bench 'DiskQueueGet' -test.benchtime 0.1
// (so that it does not perform too many iterations)
func BenchmarkDiskQueueGet(b *testing.B) {
	b.StopTimer()
	dqName := "bench_disk_queue_get" + strconv.Itoa(b.N) + strconv.Itoa(int(time.Now().Unix()))
	dq := NewDiskQueue(dqName, os.TempDir(), 1024768, 2500, 2*time.Second)
	for i := 0; i < b.N; i++ {
		dq.Put([]byte("aaaaaaaaaaaaaaaaaaaaaaaaaaa"))
	}
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		<-dq.ReadChan()
	}
}
