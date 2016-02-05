package utils

import (
	"github.com/garyburd/redigo/redis"
	"log"
)

func NewRedisPool(address string, password string) *redis.Pool {
	return &redis.Pool{
		MaxIdle:   80,
		MaxActive: 12000,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", address)

			if err != nil {
				log.Fatal(err)
			}

			if password != "" {
				if _, err := c.Do("AUTH", password); err != nil {
					c.Close()
					return nil, err
				}
			}

			return c, err
		},
	}
}
