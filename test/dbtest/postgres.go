// Package dbtest 提供测试用数据库连接管理。
package dbtest

import (
	"fmt"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// logLevelFromEnv 从 DB_LOG_LEVEL 环境变量读取日志级别。
// 可选值: silent, error, warn, info (默认: info)
func logLevelFromEnv() logger.LogLevel {
	switch os.Getenv("DB_LOG_LEVEL") {
	case "silent":
		return logger.Silent
	case "error":
		return logger.Error
	case "warn":
		return logger.Warn
	case "info":
		return logger.Info
	default:
		return logger.Info
	}
}

// newTestLogger 创建一个写入测试输出的 GORM 日志器。
// 使用 log.New 配合 testing.TB.Log 包装，使 SQL 日志出现在 `go test -v` 的对应测试节点下。
func newTestLogger(t testing.TB) logger.Interface {
	t.Helper()

	return logger.New(
		log.New(testWriter{t}, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logLevelFromEnv(),
			IgnoreRecordNotFoundError: false,
			ParameterizedQueries:      false,
			Colorful:                  true,
		},
	)
}

// testWriter 将日志写入 testing.TB.Log，使 SQL 日志与测试输出关联。
type testWriter struct {
	t testing.TB
}

func (w testWriter) Write(p []byte) (n int, err error) {
	w.t.Log(string(p))
	return len(p), nil
}

// MustDB 返回一个测试用的 GORM DB 实例，连接失败时直接 Fatal。
// SQL 日志输出到 `go test -v` 的测试节点下，日志级别由 DB_LOG_LEVEL 控制。
//
// 连接参数从环境变量读取，默认值适用于本地开发环境：
//   - DB_HOST       (默认: localhost)
//   - DB_PORT       (默认: 5432)
//   - DB_USER       (默认: postgresql)
//   - DB_PASSWORD   (默认: root)
//   - DB_NAME       (默认: orm_test)
//   - DB_SSLMODE    (默认: disable)
//   - DB_LOG_LEVEL  (默认: info，可选: silent/error/warn/info)
func MustDB(t testing.TB) *gorm.DB {
	t.Helper()

	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "postgresql")
	password := getEnv("DB_PASSWORD", "root")
	dbname := getEnv("DB_NAME", "orm_test")
	sslmode := getEnv("DB_SSLMODE", "disable")

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=Asia/Shanghai",
		host, port, user, password, dbname, sslmode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: newTestLogger(t),
	})
	if err != nil {
		t.Fatalf("连接测试数据库失败: %v (DSN: host=%s port=%s user=%s dbname=%s)", err, host, port, user, dbname)
	}
	return db
}

// MustTxDB 创建一个事务级别的 DB，所有操作在测试结束时自动回滚，
// 确保测试之间互不干扰。
func MustTxDB(t testing.TB) *gorm.DB {
	t.Helper()

	db := MustDB(t)
	tx := db.Begin()
	t.Cleanup(func() {
		tx.Rollback()
	})
	return tx
}

// SuiteDB 是一个可重用的数据库连接，支持在同一个二进制中的多个测试间共享。
// 适用于需要真实提交的场景（如事务测试）。
var (
	suiteDB   *gorm.DB
	suiteOnce sync.Once
	suiteErr  error
)

// SuiteDB 返回一个全局共享的测试数据库连接。
// 注意：SuiteDB 无法绑定到某个具体测试的 TB，因此其日志直接写入 os.Stdout。
// 建议日志级别通过 DB_LOG_LEVEL 环境变量统一控制。
func SuiteDB(t testing.TB) *gorm.DB {
	t.Helper()

	suiteOnce.Do(func() {
		suiteDB = MustDB(t)
	})
	if suiteDB == nil {
		t.Fatalf("全局测试数据库连接失败: %v", suiteErr)
	}
	return suiteDB
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// MustRawDB 返回一个不经 testing.TB 包装的裸 GORM DB（日志直接写入 stdout）。
// 用于测试框架初始化等不适合绑定 testing.TB 的场景。
func MustRawDB() (*gorm.DB, error) {
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "postgresql")
	password := getEnv("DB_PASSWORD", "root")
	dbname := getEnv("DB_NAME", "orm_test")
	sslmode := getEnv("DB_SSLMODE", "disable")

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=Asia/Shanghai",
		host, port, user, password, dbname, sslmode,
	)

	return gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevelFromEnv()),
	})
}
