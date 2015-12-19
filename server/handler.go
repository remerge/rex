package server

type Handler interface {
	Handle(*Connection)
}
