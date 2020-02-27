package gocore

import (
	"github.com/go-redis/redis/v7"
)

type AppRedis struct {
	Client 						*redis.Client
	pool						*Pool

	OnMessage 					func(msg *redis.Message)
}

// address format: localhost:6379
func NewRedisApp(address string, password string) *AppRedis {
	instance := &AppRedis{}
	instance.Client = redis.NewClient(&redis.Options{
		Addr:     address,
		Password: password, // no password set
		DB:       0,  // use default DB
	})
	instance.pool = NewPool(128, 1, 1)
	instance.OnMessage = func(msg *redis.Message){}
	return instance
}

func (this*AppRedis) Subscribe(channels ...string) {
	go this.internalSubscribe(channels...)
}

func (this*AppRedis) internalSubscribe(channels ...string) {
	pubsub := this.Client.Subscribe(channels...)
	// Wait for confirmation that subscription is created
	_, err := pubsub.Receive()
	if err != nil {
		Log().Error().Interface("channels", channels).Err(err).Msg("Error when subscribe channels")
		return
	}

	// get the message channel
	msgChannel := pubsub.Channel()

	Log().Info().Interface("channels", channels).Msg("Subscribed to channel")

	// loop for message
	for msg := range msgChannel {
		// we process message on pool so we need copy the message data
		// it will be Data Race if we use message pointer here because of message can be write when it reading on schedule
		cM := *msg
		this.pool.Schedule(func() {
			this.OnMessage(&cM)
		})
	}

	Log().Info().Interface("channels", channels).Msg("Closed pub/sub")
	// close on finish
	_ = pubsub.Close()
}