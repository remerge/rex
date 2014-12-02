package rex

/*
#include <unistd.h>
*/
import "C"

import (
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/rcrowley/go-metrics"
)

var procMetrics struct {
	cpu struct {
		user   Derive
		system Derive
	}
	vmem struct {
		size   metrics.Histogram
		rss    metrics.Histogram
		minflt Derive
		majflt Derive
	}
	stackSize metrics.Histogram
	io        struct {
		rchar Derive
		wchar Derive
		syscr Derive
		syscw Derive
	}
}

func initProcMetrics() {
	procMetrics.cpu.user = NewDerive("proc.cpu.user")
	procMetrics.cpu.system = NewDerive("proc.cpu.system")

	procMetrics.vmem.size = NewHistogram("proc.vmem.size")
	procMetrics.vmem.rss = NewHistogram("proc.vmem.rss")
	procMetrics.vmem.minflt = NewDerive("proc.vmem.minflt")
	procMetrics.vmem.majflt = NewDerive("proc.vmem.majflt")

	procMetrics.stackSize = NewHistogram("proc.stack_size")

	procMetrics.io.rchar = NewDerive("proc.io.rchar")
	procMetrics.io.wchar = NewDerive("proc.io.wchar")
	procMetrics.io.syscr = NewDerive("proc.io.syscr")
	procMetrics.io.syscw = NewDerive("proc.io.syscw")
}

func CollectProcMetrics() {
	initProcMetrics()
	for {
		time.Sleep(1e9)
		collectProcStat()
		collectProcIO()
	}
}

func collectProcStat() {
	content, err := ioutil.ReadFile("/proc/self/stat")
	if err != nil {
		//l.Errorf("failed to read from /proc/self/stat: %s", err)
		return
	}

	idx := strings.LastIndex(string(content), ")")
	fields := strings.Fields(string(content[idx+2 : len(content)]))

	minflt, _ := strconv.ParseInt(fields[7], 10, 64)
	majflt, _ := strconv.ParseInt(fields[9], 10, 64)
	utime, _ := strconv.ParseInt(fields[11], 10, 64)
	stime, _ := strconv.ParseInt(fields[12], 10, 64)
	vsize, _ := strconv.ParseInt(fields[20], 10, 64)
	rss, _ := strconv.ParseInt(fields[21], 10, 64)
	startstack, _ := strconv.ParseInt(fields[25], 10, 64)
	kstkesp, _ := strconv.ParseInt(fields[26], 10, 64)

	clkTck := int64(C.sysconf(C._SC_CLK_TCK))

	procMetrics.cpu.user.Mark(utime / clkTck)
	procMetrics.cpu.system.Mark(stime / clkTck)

	procMetrics.vmem.size.Update(vsize)
	procMetrics.vmem.rss.Update(rss)

	procMetrics.vmem.minflt.Mark(minflt)
	procMetrics.vmem.majflt.Mark(majflt)

	var stackSize int64
	if startstack > kstkesp {
		stackSize = startstack - kstkesp
	} else {
		stackSize = kstkesp - startstack
	}

	procMetrics.stackSize.Update(stackSize)
}

func collectProcIO() {
	content, err := ioutil.ReadFile("/proc/self/io")
	if err != nil {
		//l.Errorf("failed to read from /proc/self/stat: %s", err)
		return
	}

	var rchar, wchar, syscr, syscw int64

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}
		switch fields[0] {
		case "rchar:":
			rchar, _ = strconv.ParseInt(fields[1], 10, 64)
		case "wchar:":
			wchar, _ = strconv.ParseInt(fields[1], 10, 64)
		case "syscr:":
			syscr, _ = strconv.ParseInt(fields[1], 10, 64)
		case "syscw:":
			syscw, _ = strconv.ParseInt(fields[1], 10, 64)
		}
	}

	procMetrics.io.rchar.Mark(rchar)
	procMetrics.io.wchar.Mark(wchar)
	procMetrics.io.syscr.Mark(syscr)
	procMetrics.io.syscw.Mark(syscw)
}
