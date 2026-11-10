// Package database รวมของที่ทุก module ใช้ร่วมกันเกี่ยวกับ gorm:
// การเปิด connection และ transaction ที่ส่งผ่าน context
package database

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Config struct {
	Connection string // ตอนนี้รองรับ "postgres"
	Host       string
	Port       string
	User       string
	Password   string
	Name       string
	SSLMode    string
}

func (c Config) DSN() string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.User, c.Password),
		Host:     net.JoinHostPort(c.Host, c.Port),
		Path:     "/" + c.Name,
		RawQuery: url.Values{"sslmode": {c.SSLMode}, "TimeZone": {"UTC"}}.Encode(),
	}
	return u.String()
}

func Open(cfg Config) (*gorm.DB, error) {
	switch strings.ToLower(cfg.Connection) {
	case "postgres", "postgresql", "pgsql":
	default:
		return nil, fmt.Errorf("database: unsupported connection %q", cfg.Connection)
	}
	return OpenPostgres(cfg.DSN())
}

func OpenPostgres(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		TranslateError: true, // แปลง unique/FK violation เป็น gorm.ErrDuplicatedKey / ErrForeignKeyViolated
		NowFunc:        func() time.Time { return time.Now().UTC() },
		Logger: logger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true, // not found เป็นเรื่องปกติ (repository แปลงเป็น domain error)
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("database: open: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	return db, nil
}

// ---------- Transaction ผ่าน context ----------

type txKey struct{}

// TxManager implement application.TxManager ด้วย gorm
type TxManager struct{ db *gorm.DB }

func NewTxManager(db *gorm.DB) *TxManager { return &TxManager{db: db} }

// WithinTx เปิด transaction ใหม่ หรือใช้ของเดิมถ้า ctx มี transaction อยู่แล้ว (nested = join)
func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return Transact(ctx, m.db, func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

// Conn คืน *gorm.DB ที่ผูกกับ transaction ใน ctx (ถ้ามี) ไม่งั้นคืน db ปกติ
// repository ทุกตัวต้องเรียก Conn แทนการใช้ db ตรงๆ
func Conn(ctx context.Context, db *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx.WithContext(ctx)
	}
	return db.WithContext(ctx)
}

// Transact รัน fn ใน transaction — ถ้า ctx มี transaction อยู่แล้วจะ join ของเดิม
// ใช้ใน repository ที่ต้องเขียนหลายตาราง (aggregate + children) ให้ atomic
func Transact(ctx context.Context, db *gorm.DB, fn func(tx *gorm.DB) error) error {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return fn(tx.WithContext(ctx))
	}
	return db.WithContext(ctx).Transaction(fn)
}

// ---------- แปลง error ของ DB ----------

func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.Is(err, gorm.ErrDuplicatedKey) || (errors.As(err, &pgErr) && pgErr.Code == "23505")
}

func IsForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.Is(err, gorm.ErrForeignKeyViolated) || (errors.As(err, &pgErr) && pgErr.Code == "23503")
}

func IsNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }
