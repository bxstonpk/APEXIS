package entities

import (
	"fmt"

	domain "apexis/backend/internal/modules/manufacturing/domain/errors"
	vo "apexis/backend/internal/modules/manufacturing/domain/value_objects"
)

// Settings คือค่าตั้งการผลิตของบริษัท
//
//   - JournalID: สมุดรายวันที่ใช้ลงรายการต้นทุนการผลิต
//   - WipAccountID: งานระหว่างผลิต (เดบิตตอนเบิกวัตถุดิบ / ปันค่าแรง, เครดิตตอนรับสินค้าสำเร็จรูป)
//   - ConversionAccountID: ค่าแรงและค่าใช้จ่ายการผลิตปันเข้างาน (เครดิตตอนปิดงาน เมื่อบรรทัดในสูตรไม่ได้ระบุบัญชี)
//   - DefaultWarehouseID: คลังเริ่มต้นของใบสั่งผลิตใหม่ (ไม่บังคับ)
//
// ใช้ลงบัญชีเฉพาะปีบัญชีที่ใช้ระบบต่อเนื่อง (perpetual) — ระบบสิ้นงวด (periodic) มีแค่การเคลื่อนไหวสต็อก / ต้นทุน FIFO
type Settings struct {
	JournalID           *vo.JournalID
	WipAccountID        *vo.AccountID
	ConversionAccountID *vo.AccountID
	DefaultWarehouseID  *vo.WarehouseID
}

// AccountIDs คือบัญชีทั้งหมดที่ตั้งไว้ (ใช้ตรวจกับ ledger)
func (s Settings) AccountIDs() []vo.AccountID {
	var out []vo.AccountID
	for _, p := range []*vo.AccountID{s.WipAccountID, s.ConversionAccountID} {
		if p != nil {
			out = append(out, *p)
		}
	}
	return out
}

// RequirePosting ตรวจค่าตั้งที่ต้องใช้ลงบัญชี (ระบบต่อเนื่อง): สมุดรายวัน + งานระหว่างผลิต
// และบัญชีปันเข้างานเมื่อมีค่าแรง / ค่าใช้จ่ายที่ไม่ได้ระบุบัญชี
func (s Settings) RequirePosting(needConversion bool) error {
	switch {
	case s.JournalID == nil:
		return fmt.Errorf("%w: journal_id", domain.ErrSettingsMissing)
	case s.WipAccountID == nil:
		return fmt.Errorf("%w: wip_account_id", domain.ErrSettingsMissing)
	case needConversion && s.ConversionAccountID == nil:
		return fmt.Errorf("%w: conversion_account_id", domain.ErrSettingsMissing)
	}
	return nil
}
