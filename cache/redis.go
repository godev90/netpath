package cache

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Addr     string // host:port
	Username string
	Password string
	DB       int

	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PoolSize     int
}

type cachePool struct {
	pool map[string]*redis.Client
	mu   sync.RWMutex
}

var (
	pool *cachePool
	once sync.Once
)

func Pool() *cachePool {
	once.Do(func() {
		pool = &cachePool{
			pool: make(map[string]*redis.Client),
		}
	})

	return pool
}

func (rc *cachePool) Connect(alias string, cfg RedisConfig) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if _, exists := rc.pool[alias]; exists {
		return nil
	}

	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 5 * time.Second
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 3 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 3 * time.Second
	}
	if cfg.PoolSize == 0 {
		cfg.PoolSize = 10
	}

	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Username:     cfg.Username,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		PoolSize:     cfg.PoolSize,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return fmt.Errorf("failed to connect to Redis [%s]: %w", alias, err)
	}

	rc.pool[alias] = client
	log.Printf("Connected to Redis [%s]", alias)
	return nil
}

func (rc *cachePool) Get(alias string) (*redis.Client, error) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	client, ok := rc.pool[alias]
	if !ok {
		return nil, errors.New("no Redis alias found")
	}
	return client, nil
}
