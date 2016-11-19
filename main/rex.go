package main

import (
	"io"

	"github.com/remerge/rex"
	"github.com/remerge/rex/server"
	"github.com/remerge/rex/service"
	"github.com/spf13/cobra"
)

func main() {
	service := service.NewService("rex", 9990)

	service.Command.Run = func(cmd *cobra.Command, args []string) {
		go func() {
			service.Run()
			rex.MayPanic(initEchoServer())
		}()
		service.Wait(service.Shutdown)
	}

	service.Execute()
}

func initEchoServer() error {
	server, err := server.NewServer(9991)
	if err != nil {
		return err
	}

	server.Handler = &echoHandler{}

	return server.Run()
}

type echoHandler struct{}

func (h *echoHandler) Handle(c *server.Connection) {
	for {
		line, err := c.Buffer.ReadSlice('\n')
		if err != nil {
			switch err {
			case io.EOF:
				c.Close()
				return
			default:
				panic(err)
			}
		}

		c.Buffer.Write(line)

		// flush write buffer after response has been written
		if err = c.Buffer.Flush(); err != nil {
			panic(err)
		}
	}
}
