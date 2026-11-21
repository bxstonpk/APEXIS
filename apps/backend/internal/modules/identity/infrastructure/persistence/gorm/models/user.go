// Package models คือ "รูปร่างตาราง" สำหรับ gorm เท่านั้น — ไม่มี business rule
// และไม่ถูกส่งออกไปนอก infrastructure (แปลงไป/กลับ entity ใน package mapper)
//
// CreatedAt / UpdatedAt เป็นเรื่องของ persistence ล้วน — gorm set ให้อัตโนมัติ
// (autoCreateTime / autoUpdateTime) โดย domain entity ไม่ต้องรู้
package models

import (
	"time"

	"github.com/google/uuid"
)

type UserModel struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	Email        string    `gorm:"size:320;not null;uniqueIndex:uq_identity_users_email"`
	Name         string    `gorm:"size:200;not null"`
	PasswordHash string    `gorm:"size:255;not null"`
	Status       string    `gorm:"size:20;not null"`
	// รหัสผ่านชั่วคราว (ผู้ดูแลสร้างบัญชีให้) — ต้องเปลี่ยนก่อนใช้งาน
	MustChangePassword bool      `gorm:"not null;default:false"`
	CreatedAt          time.Time `gorm:"not null"`
	UpdatedAt          time.Time `gorm:"not null"`
}

func (UserModel) TableName() string { return "identity_users" }
