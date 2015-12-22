package main

import (
	"github.com/remerge/rex"
)

func main() {
	service := &rex.Service{BaseConfig: rex.NewConfig("rex", 9990)}
	service.Init()
	go service.Run()
	service.Wait(service.Shutdown)
}
