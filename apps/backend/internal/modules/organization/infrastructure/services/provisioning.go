package services

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/organization/application/ports"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
	"apexis/backend/internal/platform/integration"
)

var (
	_ ports.CompanyProvisioner = (*Provisioning)(nil)
	_ ports.FiscalYearListener = (*Provisioning)(nil)
)

// Provisioning คือรายการ hook ของ module อื่นที่ตั้งค่าเริ่มต้นให้บริษัทใหม่
// ลงทะเบียนตอนเริ่มโปรแกรม (main.go) แล้วเรียกตามลำดับที่ลงทะเบียน ภายใน transaction ของการสร้างบริษัท
// ลำดับสำคัญ: ledger (ผังบัญชี) ต้องมาก่อน module ที่ตั้งบัญชีในค่าตั้งของตัวเอง
type Provisioning struct {
	mu          sync.RWMutex
	hooks       []integration.CompanyCreatedHook
	fiscalHooks []integration.FiscalYearCreatedHook
}

// AddFiscal ลงทะเบียน hook ที่เรียกหลังสร้างปีบัญชี (ภายใน transaction ของการสร้างปีบัญชี)
func (p *Provisioning) AddFiscal(h integration.FiscalYearCreatedHook) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.fiscalHooks = append(p.fiscalHooks, h)
}

func (p *Provisioning) FiscalYearCreated(ctx context.Context, companyID vo.CompanyID, year int) error {
	p.mu.RLock()
	hooks := append([]integration.FiscalYearCreatedHook(nil), p.fiscalHooks...)
	p.mu.RUnlock()
	for _, h := range hooks {
		if err := h(ctx, uuid.UUID(companyID), year); err != nil {
			return err
		}
	}
	return nil
}

func NewProvisioning() *Provisioning { return &Provisioning{} }

func (p *Provisioning) Add(h integration.CompanyCreatedHook) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.hooks = append(p.hooks, h)
}

func (p *Provisioning) Provision(ctx context.Context, companyID vo.CompanyID, businessType vo.BusinessType) error {
	p.mu.RLock()
	hooks := append([]integration.CompanyCreatedHook(nil), p.hooks...)
	p.mu.RUnlock()
	c := integration.NewCompany{ID: uuid.UUID(companyID), BusinessType: integration.BusinessType(businessType)}
	for _, h := range hooks {
		if err := h(ctx, c); err != nil {
			return err
		}
	}
	return nil
}
