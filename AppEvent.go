package gocore

import (
	"time"
)
type AppEvent struct {}
var Bus = &AppEvent{}

func(b*AppEvent) CallAfter(dur time.Duration, repeat int, callback func(called int)){
	go func() {
		count := 0
		for range time.Tick(dur) {
			callback(count)
			if repeat < 0 {
				continue
			}else{
				count++
				if count >= repeat {
					return
				}
			}
		}
	}()
}

