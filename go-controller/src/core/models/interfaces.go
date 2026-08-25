package models

type Session interface {
	Done() <-chan struct{}
	Close() error
}
