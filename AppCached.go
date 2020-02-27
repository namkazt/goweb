package gocore

import (
	"time"
	"github.com/akyoto/cache"
)

type AppCached struct {
	items 				*cache.Cache
}
var _sharedAppCached *AppCached
func Cache() *AppCached {
	if _sharedAppCached == nil {
		_sharedAppCached = &AppCached{
			items: cache.New(6 * time.Hour),
		}
	}
	return _sharedAppCached
}


func(c*AppCached) Has(key interface{}) bool {
	_, found := c.items.Get(key)
	return found
}

func(c*AppCached) Get(key interface{}) CacheAny {
	obj, found := c.items.Get(key)
	if found {
		return as(obj)
	}
	return CacheAny{}
}


func(c*AppCached) Delete(key interface{}) {
	c.items.Delete(key)
}

func(c*AppCached) Set(key interface{}, value interface{}, dur time.Duration) {
	c.items.Set(key, value, dur)
}

type CacheAny struct {
	value 			interface{}
	good			bool
}

func as(value interface{}) CacheAny {
	return CacheAny{
		value: value,
		good: true,
	}
}

func (a CacheAny) String() string{
	if(!a.good) { return "" }
	return a.value.(string)
}

func (a*CacheAny) Int8() int8{
	if(!a.good) { return 0 }
	return a.value.(int8)
}

func (a CacheAny) Int16() int16{
	if(!a.good) { return 0 }
	return a.value.(int16)
}

func (a*CacheAny) Int32() int32{
	if(!a.good) { return 0 }
	return a.value.(int32)
}

func (a CacheAny) Int64() int64{
	if(!a.good) { return 0 }
	return a.value.(int64)
}

func (a CacheAny) Int() int{
	if(!a.good) { return 0 }
	return a.value.(int)
}

func (a CacheAny) Float64() float64{
	if(!a.good) { return 0 }
	return a.value.(float64)
}

func (a CacheAny) Float32() float32{
	if(!a.good) { return 0 }
	return a.value.(float32)
}

func (a CacheAny) Bool() bool{
	if(!a.good) { return false }
	return a.value.(bool)
}

func (a CacheAny) Byte() byte{
	if(!a.good) { return 0 }
	return a.value.(byte)
}

func (a CacheAny) Rune() rune{
	if(!a.good) { return 0 }
	return a.value.(rune)
}

func (a CacheAny) As(data interface{}){
	if(!a.good) { return }
	data = a.value
}

func (a CacheAny) V() interface{}{
	return a.value
}