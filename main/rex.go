package main

import (
	"github.com/remerge/rex/service"
	"github.com/spf13/cobra"
)

func main() {
	service := service.NewService("rex", 9990)

	service.Command.Run = func(cmd *cobra.Command, args []string) {
		go service.Run()
		service.Wait(service.Shutdown)
	}

	service.Execute()
}
