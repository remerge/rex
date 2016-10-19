package main

import "github.com/remerge/rex/service"

func main() {
	service.NewService("rex", 9990).Execute()
}
