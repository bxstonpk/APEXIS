package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"apexis/backend/internal/modules/finance"
	"apexis/backend/internal/modules/identity"
	"apexis/backend/internal/modules/inventory"
	"apexis/backend/internal/modules/ledger"
	"apexis/backend/internal/modules/organization"
	"apexis/backend/internal/modules/purchasing"
	"apexis/backend/internal/modules/sales"
	"apexis/backend/internal/modules/taxregistry"
	"apexis/backend/internal/platform/config"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/permissions"
	"apexis/backend/internal/platform/reports"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	// รันได้ทั้งจาก apps/backend และจาก root ของ repo
	if err := config.LoadDotEnv(".env", "../../.env"); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := database.Open(cfg.Database)
	if err != nil {
		return err
	}
	if err := identity.Migrate(db); err != nil {
		return err
	}
	if err := organization.Migrate(db); err != nil {
		return err
	}
	if err := ledger.Migrate(db); err != nil {
		return err
	}
	if err := inventory.Migrate(db); err != nil {
		return err
	}
	if err := finance.Migrate(db); err != nil {
		return err
	}
	// ผู้ขายรหัสซ้ำ (เลขผู้เสียภาษีเดียวกัน) ถูกรวม — ใบจ่ายเงินของ finance ต้องชี้รหัสที่เก็บไว้
	supplierMerges, err := purchasing.Migrate(db)
	if err != nil {
		return err
	}
	if err := finance.RemapParties(db, "supplier", supplierMerges); err != nil {
		return err
	}
	// ลูกค้ารหัสซ้ำ (เลขผู้เสียภาษีเดียวกัน) ถูกรวม — ใบรับเงินของ finance ต้องชี้รหัสที่เก็บไว้
	customerMerges, err := sales.Migrate(db)
	if err != nil {
		return err
	}
	if err := finance.RemapParties(db, "customer", customerMerges); err != nil {
		return err
	}
	// สิทธิ์เดิมของบทบาทตามความรับผิดชอบที่ย้าย module (รับ / จ่ายเงิน -> finance) — ทำซ้ำได้
	renames := make([]identity.PermissionRename, 0, len(finance.PermissionMigrations()))
	for _, r := range finance.PermissionMigrations() {
		renames = append(renames, identity.PermissionRename{From: r.From, To: r.To, Keep: r.Keep})
	}
	if err := identity.RenamePermissions(db, renames); err != nil {
		return err
	}

	identityModule, err := identity.New(db, identity.Config{
		JWTSecret: []byte(cfg.JWTSecret),
		TokenTTL:  cfg.JWTTTL,
		JWTIssuer: cfg.JWTIssuer,
		Logger:    logger,
	})
	if err != nil {
		return err
	}

	if err := seedInitialAdmin(identityModule, cfg.InitialAdmin, logger); err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	identityModule.RegisterRoutes(mux)
	// module อื่นของ ERP ลงทะเบียนตรงนี้ และพึ่งพา identity ผ่าน public API ของ identityModule
	organizationModule := organization.New(db, identityModule)
	organizationModule.RegisterRoutes(mux)
	ledgerModule := ledger.New(db, identityModule, organizationModule)
	ledgerModule.RegisterRoutes(mux)
	inventoryModule := inventory.New(db, identityModule, organizationModule, ledgerModule)
	inventoryModule.RegisterRoutes(mux)
	purchasingModule := purchasing.New(db, identityModule, organizationModule, ledgerModule, inventoryModule)
	purchasingModule.RegisterRoutes(mux)
	salesModule := sales.New(db, identityModule, organizationModule, ledgerModule, inventoryModule)
	salesModule.RegisterRoutes(mux)
	// การเงินเป็นเจ้าของเงินเข้า-ออกทั้งหมด — ตัดยอดใบแจ้งหนี้ผ่าน public API ของ sales / purchasing
	financeModule := finance.New(db, identityModule, organizationModule, ledgerModule, salesModule, purchasingModule)
	financeModule.RegisterRoutes(mux)
	// ค้นหาผู้จด VAT จากกรมสรรพากร (เติมฟอร์มลูกค้า / ผู้ขาย) — ใช้ได้เมื่อจัดการลูกค้าหรือผู้ขายได้
	taxregistry.New(identityModule, cfg.VatRegistryURL, []string{"sales.customer:manage", "purchasing.supplier:manage"}).RegisterRoutes(mux)

	// รายงานทุก module ผ่าน endpoint เดียว: GET /reports?book=... (หน้ารายงานของ frontend เป็นหน้าเดียว)
	reportBooks := reports.NewRegistry()
	ledgerModule.RegisterReports(reportBooks)
	inventoryModule.RegisterReports(reportBooks)
	purchasingModule.RegisterReports(reportBooks)
	salesModule.RegisterReports(reportBooks)
	financeModule.RegisterReports(reportBooks)
	mux.Handle("GET /api/v1/companies/{companyID}/reports", identityModule.Authenticate(reportBooks))

	// รายการสิทธิ์ทั้งหมด (หน้ากำหนดบทบาทของ frontend) — แต่ละ module ลงทะเบียนของตัวเอง
	permissionCatalog := permissions.NewCatalog()
	for _, m := range []interface{ Permissions() []string }{
		salesModule, purchasingModule, inventoryModule, ledgerModule, financeModule, organizationModule, identityModule,
	} {
		permissionCatalog.Register(m.Permissions()...)
	}
	mux.Handle("GET /api/v1/permissions", identityModule.Authenticate(permissionCatalog))

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("http server listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// initialAdminName คือชื่อที่แสดงของผู้ดูแลระบบที่ seed (แก้ได้ภายหลังในระบบ)
const initialAdminName = "Administrator"

// seedInitialAdmin สร้างผู้ดูแลระบบจาก INITIAL_ADMIN_* ถ้ายังไม่มี — ไม่ตั้งค่า = ข้าม
// ผู้ใช้ที่ active สร้างบริษัทได้และเป็น owner (สิทธิ์ทุกอย่าง) ของบริษัทที่สร้าง
func seedInitialAdmin(identityModule *identity.Module, admin config.InitialAdmin, logger *slog.Logger) error {
	if admin.Email == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	created, err := identityModule.SeedUser(ctx, admin.Email, initialAdminName, admin.Password)
	if err != nil {
		return fmt.Errorf("seed initial admin (INITIAL_ADMIN_USERNAME must be an email, INITIAL_ADMIN_PASSWORD 8-72 characters): %w", err)
	}
	if created {
		logger.Info("initial admin created", "email", admin.Email)
	} else {
		logger.Info("initial admin already exists", "email", admin.Email)
	}
	return nil
}
