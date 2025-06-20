package database

import (
	"fmt"
	"log"
	"sync"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Config struct {
	Driver   string
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

var (
	connections = make(map[string]*gorm.DB)
	configs     = make(map[string]Config)
	mu          sync.RWMutex
)

func Connect(name string) {
	mu.Lock()
	defer mu.Unlock()

	if _, exists := connections[name]; exists {
		return // already connected
	}

	cfg, ok := configs[name]
	if !ok {
		log.Fatalf("No DB config found for name: %s", name)
	}

	var dsn string
	var dialector gorm.Dialector

	switch cfg.Driver {
	case "mysql":
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name)
		dialector = mysql.Open(dsn)

	case "postgres":
		dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name)
		dialector = postgres.Open(dsn)

	default:
		log.Fatalf("Unsupported DB driver: %s", cfg.Driver)
		return
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("Failed to connect to [%s] database: %v", name, err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get sql.DB from gorm DB: %v", err)
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(1 * time.Hour)

	connections[name] = db
	log.Printf("Connected to [%s] database", name)
}

func Get(name string) *gorm.DB {
	mu.RLock()
	defer mu.RUnlock()

	db, ok := connections[name]
	if !ok {
		log.Fatalf("No DB connection found with name: %s", name)
	}
	return db
}

func AddConnection(name string, config Config) {
	mu.Lock()
	defer mu.Unlock()
	configs[name] = config
}
