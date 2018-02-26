package models

import (
	"github.com/go-redis/redis"
)

//to be globally accessable by multiple routes
var client *redis.Client

//Init serves clients from redis ??? not sure advantage over direct
func Init() {
	client = redis.NewClient(&redis.Options{
		Addr: "localhost:6379", //default port of redis-server; lo-host when same machine
	})
}
