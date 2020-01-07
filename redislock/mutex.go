package redislock

import (
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/go-redis/redis"
)

// A DelayFunc is used to decide the amount of time to wait between retries.
type DelayFunc func(tries int) time.Duration

type Mutex struct {
	name         string //redis key值
	expiry       time.Duration
	tries        int
	delayFunc    DelayFunc
	factor       float64
	quorum       int
	genValueFunc func() (string, error)
	value        string
	until        time.Time
	clients      []*redis.Client
}

func (m *Mutex) Lock() error {
	value, err := m.genValueFunc()
	if err != nil {
		return err
	}

	for i := 0; i < m.tries; i++ {
		if i != 0 {
			time.Sleep(m.delayFunc(i))
		}

		start := time.Now()
		n := m.actOnClientsAsync(func(client *redis.Client) bool {
			return m.acquire(client, value)
		})

		now := time.Now()
		until := now.Add(m.expiry - now.Sub(start) - time.Duration(int64(float64(m.expiry)*m.factor)))
		if n >= m.quorum && now.Before(until) {
			m.value = value
			m.until = until
			return nil
		}
		m.actOnClientsAsync(func(client *redis.Client) bool {
			return m.release(client, value)
		})
	}
	return ErrFailed
}

func (m *Mutex) Unlock() bool {
	n := m.actOnClientsAsync(func(client *redis.Client) bool {
		return m.release(client, m.value)
	})
	return n >= m.quorum
}

func (m *Mutex) Extend() bool {
	n := m.actOnClientsAsync(func(client *redis.Client) bool {
		return m.touch(client, m.value, int(m.expiry/time.Millisecond))
	})
	return n >= m.quorum
}

func genValue() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

func (m *Mutex) acquire(client *redis.Client, value string) bool {
	lockReply, err := client.SetNX(m.name, value, m.expiry).Result()
	return err == nil && lockReply == true
}

var deleteScript = redis.NewScript(`
    if redis.call("GET", KEYS[1]) == ARGV[1] then
        return redis.call("DEL", KEYS[1])
    else
        return 0
    end
`)

func (m *Mutex) release(client *redis.Client, value string) error {
	return deleteScript.Run(client, []string{m.name}, value).Err()
}

var touchScript = redis.NewScript(`
    if redis.call("GET", KEY[1]) == ARGV[1] then
        return redis.call("pexpire", KEYS[1], ARGV[2])
    else
        return 0
    end
`)

//touch expiry毫秒
func (m *Mutex) touch(client *redis.Client, value string, expiry int) error {
	return touchScript.Run(client, []string{m.name}, value, expiry).Err()
}

func (m *Mutex) actOnClientsAsync(actFun func(*redis.Client) bool) int {
	ch := make(chan bool)
	for _, client := range m.clients {
		go func(*redis.Client) {
			ch <- actFun(client)
		}(client)
	}

	n := 0
	for range m.clients {
		if <-ch {
			n++
		}
	}
	return n
}
