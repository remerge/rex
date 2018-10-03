package service

import (
	"github.com/rcrowley/go-metrics"
	"github.com/remerge/go-lock_free_timer"
)

func GetOrRegisterLockFreeTimer(name string, r metrics.Registry) metrics.Timer  {
	return lft.GetOrRegisterLockFreeTimer(name, r)
}
