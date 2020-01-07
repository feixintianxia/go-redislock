package redislock

import (
	"time"

	"github.com/go-redis/redis"
)

// LockGenerator 分布式锁生成器
type LockGenerator struct {
	clients []*redis.Client
}

// NewLockGenerator 基于多个redis客户端生成NewLockGenerator实例
func NewLockGenerator(clients []*redis.Client) *LockGenerator {
	return &LockGenerator{
		clients: clients,
	}
}

// NewMutex returns a new distributed mutex with given name.
func (g *LockGenerator) NewMutex(name string, options ...Option) *Mutex {
	m := &Mutex{
		name:         name,
		expiry:       8 * time.Second,
		tries:        32,
		delayFunc:    func(tries int) time.Duration { return 500 * time.Hour.Milliseconds() },
		genValueFunc: genValue,
		factor:       0.01,
		quorum:       len(r.clients)/2 + 1,
		clients:      r.clients,
	}

	for _, o := range options {
		o.Apply(m)
	}
	return m
}

// Option
type Option interface {
	Apply(*Mutex)
}

// OptionFunc is a function that configures a mutex.
type OptionFunc func(*Mutex)

// Apply calls f(mutex)
func (f OptionFunc) Apply(mutex *Mutex) {
	f(mutex)
}

// SetExpiry can be used to set the expiry of a mutex to the given value.
func SetExpiry(expiry time.Duration) Option {
	return OptionFunc(func(m *Mutex) {
		m.expiry = expiry
	})
}

// SetTries can be used to set the number of times lock acquire is attempted.
func SetTries(tries int) Option {
	return OptionFunc(func(m *Mutex) {
		m.tries = tries
	})
}

// SetRetryDelayFunc can be used to override default delay behavior.
func SetRetryDelayFunc(delayFunc DelayFunc) Option {
	return OptionFunc(func(m *Mutex) {
		m.delayFunc = delayFunc
	})
}

// SetDriftFactor can be used to set the clock drift factor.
func SetDriftFactor(factor float64) Option {
	return OptionFunc(func(m *Mutex) {
		m.factor = factor
	})
}

// SetGenValueFunc can be used to set the custom value generator.
func SetGenValueFunc(genValueFunc func() (string, error)) Option {
	return OptionFunc(func(m *Mutex) {
		m.genValueFunc = genValueFunc
	})
}
