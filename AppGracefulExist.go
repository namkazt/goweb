package gocore

import (
	"os/signal"
	"os"
	"sync"
	"syscall"
	"fmt"
)

type AppGracefulExist struct {
	gracefulMutex					sync.Mutex
	gracefulStop					chan os.Signal
	gracefulCallbacks				map[string]func()
}

func NewGracefulExist() *AppGracefulExist {
	instance := &AppGracefulExist{
		gracefulCallbacks: make(map[string]func()),
		gracefulStop: make(chan os.Signal),
	}
	return instance
}

func (this* AppGracefulExist) Start() {
	signal.Notify(this.gracefulStop, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL)
	go this.GracefulExit()
}

func (this* AppGracefulExist) GracefulExit() {
	sig := <- this.gracefulStop
	fmt.Println("[Graceful Exit] catch signal: " +  fmt.Sprintf("%+v", sig))
	this.gracefulMutex.Lock()
	for key := range this.gracefulCallbacks {
		this.gracefulCallbacks[key]()
	}
	this.gracefulMutex.Unlock()
	os.Exit(0)
}

func (this* AppGracefulExist) AddGracefulCallback(key string, callback func()) {
	this.gracefulMutex.Lock()
	this.gracefulCallbacks[key] = callback
	this.gracefulMutex.Unlock()
}

func (this* AppGracefulExist) RemoveGracefulCallback(key string) {
	this.gracefulMutex.Lock()
	delete(this.gracefulCallbacks,key)
	this.gracefulMutex.Unlock()
}
