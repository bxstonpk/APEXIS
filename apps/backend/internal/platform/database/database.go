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
//
// transaction ถูกเปิดที่ use case เท่านั้น (ผ่าน TransactionManager ของแต่ละ module)
// แล้วส่งต่อผ่าน ctx ให้ repository ทุกตัว — key อยู่ที่ platform เพื่อให้ repository
// ของหลาย module เข้าร่วม transaction เดียวกันได้

type txKey struct{}

// ErrNoTransaction: repository ที่เขียนหลายตารางถูกเรียกนอก transaction ของ use case
var ErrNoTransaction = errors.New("database: operation requires a transaction from the use case")

// WithinTransaction เปิด transaction ใหม่ หรือ join ของเดิมถ้า ctx มี transaction อยู่แล้ว
func WithinTransaction(ctx context.Context, db *gorm.DB, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return fn(ctx)
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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

// TxConn คืน transaction ใน ctx — ใช้กับการเขียนหลายตาราง (aggregate + children)
// ที่ต้อง atomic: repository ห้ามเปิด transaction เอง จึงบังคับให้ use case เปิดไว้ก่อน
func TxConn(ctx context.Context) (*gorm.DB, error) {
	tx, ok := ctx.Value(txKey{}).(*gorm.DB)
	if !ok {
		return nil, ErrNoTransaction
	}
	return tx.WithContext(ctx), nil
}

// ---------- Query helper ----------

// LikeContains คืน pattern สำหรับ LOWER(col) LIKE ? แบบ "มีคำนี้อยู่"
// escape % _ \ เพื่อให้ผู้ใช้ค้นหาตัวอักษรเหล่านี้ได้ตรงตัว (backslash เป็น escape เริ่มต้นของ PostgreSQL/MySQL)
func LikeContains(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + strings.ToLower(r.Replace(s)) + "%"
}

// ---------- แปลง error ของ DB ----------

func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.Is(err, gorm.ErrDuplicatedKey) || (errors.As(err, &pgErr) && pgErr.Code == "23505")
}

// UniqueConstraint: ชื่อ unique index ที่ถูกละเมิด (ว่าง = ไม่ใช่ / ไม่ทราบ) — ใช้แยกว่า "รหัสซ้ำ" หรือ "เลขผู้เสียภาษีซ้ำ"
func UniqueConstraint(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return pgErr.ConstraintName
	}
	return ""
}

func IsForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.Is(err, gorm.ErrForeignKeyViolated) || (errors.As(err, &pgErr) && pgErr.Code == "23503")
}

func IsNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }
