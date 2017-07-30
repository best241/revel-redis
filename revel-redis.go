// This module configures a redis connect for the application
package revelRedis

import (
	"github.com/revel/revel"
    "github.com/garyburd/redigo/redis"
	"os"
	"regexp"
	"strings"
	"strconv"
	"fmt"
	"time"
)

var (
	redisPool                 *redis.Pool
)

func Init() {
	// Read configuration.
	var found bool
	var host string
	var password string
	var port int

	// First look in the environment for REDIS_URL
	url := os.Getenv("REDIS_URL")

	// Check it matches a redis url format
	if match, _ := regexp.MatchString("^redis://(.*:.*@)?[^@]*:[0-9]+$", url); match {

		// Remove the scheme
		url = strings.Replace(url, "redis://", "", 1)
		parts := strings.Split(url, "@")

		// Smash off the credentials
		if len(parts) > 1 {
			url = parts[1]
			password = strings.Split(parts[0], ":")[1]
		}

		// Split to get the port off the end
		parts = strings.Split(url, ":")
		if len(parts) != 2 {
			revel.ERROR.Fatal(fmt.Sprintf("REDIS_URL format was incorrect (%s)", url))
		}

		// Get the host and possible password
		var port64 int64
		host = parts[0]
		port64, _ = strconv.ParseInt(parts[1], 0, 0)
		if port64 > 0{
			port = int(port64)
		}
	}

	// Then look into the configuration for redis.host and redis.port
	if len(host) == 0 {
		if host, found = revel.Config.String("redis.host"); !found {
			revel.ERROR.Fatal("No redis.host found.")
		}
	}
	if len(password) == 0 {
		password, _ = revel.Config.String("redis.password")
	}
	if port == 0 {
		if port, found = revel.Config.Int("redis.port"); !found {
			port = 6379
		}
	}

	redisPool = newRedisPool("tcp", fmt.Sprintf("%s:%d", host, port), password)

	if redisPool == nil {
		revel.ERROR.Fatal("RedisPool == nil")
	}
}

type RedisController struct {
	*revel.Controller
	RedisPool         *redis.Pool
}

func (c *RedisController) Begin() revel.Result {
	c.RedisPool = redisPool
	return nil
}

func init() {
	revel.OnAppStart(Init)
	revel.InterceptMethod((*RedisController).Begin, revel.BEFORE)
}

func (this *RedisController) DoRedis(commandName string, args ...interface{}) (reply interface{}, err error) {
	client := this.RedisPool.Get()
	defer client.Close()
	reply, err = client.Do(commandName, args...)
	return
}


func newRedisPool(protocol, server, password string) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial(protocol, server)
			if err != nil {
				return nil, err
			}
			if password != "" {
				if _, err := c.Do("AUTH", password); err != nil {
					c.Close()
					return nil, err
				}
			}
			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) (err error) {
			if time.Since(t) < 5*time.Second {
				return nil
			}
			_, err = c.Do("PING")
			return
		},
	}
}
