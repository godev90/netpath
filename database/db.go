package tools

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

type (
	DBConfig struct {
		Driver   string
		Host     string
		Port     string
		User     string
		Password string
		Name     string

		MaxOpenConns    int
		MaxIdleConns    int
		ConnMaxLifetime time.Duration
	}

	dbPool struct {
		pool map[string]*sql.DB
		mu   sync.RWMutex
	}
)

var (
	pool *dbPool
	once sync.Once
)

func Pool() *dbPool {
	once.Do(func() {
		pool = &dbPool{
			pool: make(map[string]*sql.DB),
		}
	})

	return pool
}

func (dbc *dbPool) Connect(alias string, cfg DBConfig) error {
	dbc.mu.Lock()
	defer dbc.mu.Unlock()

	if _, exists := dbc.pool[alias]; exists {
		return nil
	}

	var dsn string
	switch cfg.Driver {
	case "mysql":
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name)

	case "postgres":
		dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name)

	default:
		log.Fatalf("Unsupported DB driver: %s", cfg.Driver)
	}

	db, err := sql.Open(cfg.Driver, dsn)
	if err != nil {
		return err
	}

	if err := db.Ping(); err != nil {
		return err
	}

	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	} else {
		db.SetMaxOpenConns(25)
	}

	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	} else {
		db.SetMaxIdleConns(5)
	}

	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	} else {
		db.SetConnMaxLifetime(1 * time.Hour)
	}

	dbc.pool[alias] = db
	log.Printf("Connected to [%s] database", alias)

	return nil
}

func (dbc *dbPool) Get(name string) (*sql.DB, error) {
	dbc.mu.RLock()
	defer dbc.mu.RUnlock()

	db, ok := dbc.pool[name]
	if !ok {
		return nil, errors.New("no alias found")
	}

	return db, nil
}
