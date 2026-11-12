// Package gormpersistence คือ adapter ระหว่าง domain กับ gorm ของ inventory
//   - mapper.go       แปลง domain <-> gorm model (ที่เดียวที่รู้จักทั้งสองฝั่ง)
//   - transaction.go  implement ports.TransactionManager
//   - models/         รูปร่างตาราง
//   - repositories/   implement application/ports/repositories
package gormpersistence

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/inventory/domain/entities"
	"apexis/backend/internal/modules/inventory/domain/services"
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
	"apexis/backend/internal/modules/inventory/infrastructure/persistence/gorm/models"
)

func optUUID[T ~[16]byte](id *T) *uuid.UUID {
	if id == nil {
		return nil
	}
	u := uuid.UUID(*id)
	return &u
}

func optID[T ~[16]byte](u *uuid.UUID) *T {
	if u == nil {
		return nil
	}
	id := T(*u)
	return &id
}

func optDecimal(d *decimal.Decimal) *decimal.Decimal {
	if d == nil {
		return nil
	}
	v := *d
	return &v
}

// DateUTC: คอลัมน์ date อ่านกลับมาอาจมี location ของ driver — แปลงเป็นเที่ยงคืน UTC ตามข้อตกลงของ domain
func DateUTC(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// ---------- master ----------

func UomToModel(u *entities.Uom) models.UomModel {
	d := u.Data()
	return models.UomModel{ID: uuid.UUID(u.ID()), CompanyID: uuid.UUID(u.CompanyID()), Code: d.Code, Name: d.Name, Status: string(d.Status)}
}

func UomToEntity(m models.UomModel) *entities.Uom {
	return entities.RehydrateUom(vo.UomID(m.ID), vo.CompanyID(m.CompanyID), entities.MasterData{Code: m.Code, Name: m.Name, Status: vo.Status(m.Status)})
}

func CategoryToModel(c *entities.Category) models.CategoryModel {
	d := c.Data()
	return models.CategoryModel{ID: uuid.UUID(c.ID()), CompanyID: uuid.UUID(c.CompanyID()), Code: d.Code, Name: d.Name, Status: string(d.Status)}
}

func CategoryToEntity(m models.CategoryModel) *entities.Category {
	return entities.RehydrateCategory(vo.CategoryID(m.ID), vo.CompanyID(m.CompanyID), entities.MasterData{Code: m.Code, Name: m.Name, Status: vo.Status(m.Status)})
}

func WarehouseToModel(w *entities.Warehouse) models.WarehouseModel {
	d := w.Data()
	return models.WarehouseModel{
		ID: uuid.UUID(w.ID()), CompanyID: uuid.UUID(w.CompanyID()), Code: d.Code, Name: d.Name,
		BranchID: optUUID(d.BranchID), Status: string(d.Status),
	}
}

func WarehouseToEntity(m models.WarehouseModel) *entities.Warehouse {
	return entities.RehydrateWarehouse(vo.WarehouseID(m.ID), vo.CompanyID(m.CompanyID), entities.WarehouseData{
		Code: m.Code, Name: m.Name, BranchID: optID[vo.BranchID](m.BranchID), Status: vo.Status(m.Status),
	})
}

func ItemToModel(i *entities.Item) models.ItemModel {
	d := i.Data()
	return models.ItemModel{
		ID: uuid.UUID(i.ID()), CompanyID: uuid.UUID(i.CompanyID()), Code: d.Code, Name: d.Name, NameEN: d.NameEN,
		Type: string(d.Type), UomID: uuid.UUID(d.UomID), CategoryID: optUUID(d.CategoryID),
		ReorderPoint: optDecimal(d.ReorderPoint), Status: string(d.Status),
	}
}

func ItemToEntity(m models.ItemModel) *entities.Item {
	return entities.RehydrateItem(vo.ItemID(m.ID), vo.CompanyID(m.CompanyID), entities.ItemData{
		Code: m.Code, Name: m.Name, NameEN: m.NameEN, Type: vo.ItemType(m.Type), UomID: vo.UomID(m.UomID),
		CategoryID: optID[vo.CategoryID](m.CategoryID), ReorderPoint: optDecimal(m.ReorderPoint), Status: vo.Status(m.Status),
	})
}

// ---------- document ----------

func DocumentToModel(doc *entities.Document) models.DocumentModel {
	s := doc.State()
	m := models.DocumentModel{
		ID: uuid.UUID(s.ID), CompanyID: uuid.UUID(s.CompanyID), DocType: string(s.Data.Type), DocDate: s.Data.Date,
		WarehouseID: uuid.UUID(s.Data.WarehouseID), ToWarehouseID: optUUID(s.Data.ToWarehouseID),
		Reference: s.Data.Reference, Description: s.Data.Description, Status: string(s.Status),
		CreatedBy: uuid.UUID(s.CreatedBy), TotalValue: decimal.Zero,
	}
	if p := s.Posting; p != nil {
		no, period, by, at := p.DocumentNo, uuid.UUID(p.PeriodID), uuid.UUID(p.By), p.At
		m.DocumentNo, m.FiscalPeriodID, m.PostedBy, m.PostedAt = &no, &period, &by, &at
	}
	if c := s.Cancellation; c != nil {
		by, at := uuid.UUID(c.By), c.At
		m.CancelledBy, m.CancelledAt = &by, &at
	}
	m.Lines = make([]models.DocumentLineModel, len(s.Data.Lines))
	for i, l := range s.Data.Lines {
		m.TotalValue = m.TotalValue.Add(l.Value)
		m.Lines[i] = models.DocumentLineModel{
			DocumentID: m.ID, LineNo: i + 1, ItemID: uuid.UUID(l.ItemID), Quantity: l.Quantity,
			UnitCost: optDecimal(l.UnitCost), Value: l.Value, Description: l.Description,
		}
	}
	return m
}

func DocumentToEntity(m models.DocumentModel) *entities.Document {
	s := entities.DocumentState{
		ID: vo.DocumentID(m.ID), CompanyID: vo.CompanyID(m.CompanyID), Status: vo.DocumentStatus(m.Status),
		CreatedBy: vo.UserID(m.CreatedBy),
		Data: entities.DocumentData{
			Type: vo.DocumentType(m.DocType), Date: DateUTC(m.DocDate), WarehouseID: vo.WarehouseID(m.WarehouseID),
			ToWarehouseID: optID[vo.WarehouseID](m.ToWarehouseID), Reference: m.Reference, Description: m.Description,
			Lines: make([]entities.LineData, len(m.Lines)),
		},
	}
	if m.DocumentNo != nil && m.FiscalPeriodID != nil && m.PostedBy != nil && m.PostedAt != nil {
		s.Posting = &entities.Posting{
			DocumentNo: *m.DocumentNo, PeriodID: vo.FiscalPeriodID(*m.FiscalPeriodID), By: vo.UserID(*m.PostedBy), At: *m.PostedAt,
		}
	}
	if m.CancelledBy != nil && m.CancelledAt != nil {
		s.Cancellation = &entities.Cancellation{By: vo.UserID(*m.CancelledBy), At: *m.CancelledAt}
	}
	for i, l := range m.Lines {
		s.Data.Lines[i] = entities.LineData{
			ItemID: vo.ItemID(l.ItemID), Quantity: l.Quantity, UnitCost: optDecimal(l.UnitCost),
			Value: l.Value, Description: l.Description,
		}
	}
	return entities.RehydrateDocument(s)
}

// ---------- stock ----------

func LayerToModel(companyID vo.CompanyID, l *services.Layer) models.LayerModel {
	return models.LayerModel{
		ID: uuid.UUID(l.ID), CompanyID: uuid.UUID(companyID), ItemID: uuid.UUID(l.Key.ItemID), WarehouseID: uuid.UUID(l.Key.WarehouseID),
		LayerDate: l.Date, DocumentID: uuid.UUID(l.DocumentID), LineNo: l.LineNo, Quantity: l.Quantity, Value: l.Value,
		RemainingQuantity: l.RemainingQuantity, RemainingValue: l.RemainingValue,
	}
}

func LayerToDomain(m models.LayerModel) *services.Layer {
	return &services.Layer{
		ID: vo.LayerID(m.ID), Key: services.StockKey{ItemID: vo.ItemID(m.ItemID), WarehouseID: vo.WarehouseID(m.WarehouseID)},
		Date: DateUTC(m.LayerDate), DocumentID: vo.DocumentID(m.DocumentID), LineNo: m.LineNo,
		Quantity: m.Quantity, Value: m.Value, RemainingQuantity: m.RemainingQuantity, RemainingValue: m.RemainingValue,
	}
}

func MovementToModel(companyID vo.CompanyID, mv services.Movement) models.MovementModel {
	return models.MovementModel{
		ID: uuid.UUID(mv.ID), CompanyID: uuid.UUID(companyID), ItemID: uuid.UUID(mv.Key.ItemID), WarehouseID: uuid.UUID(mv.Key.WarehouseID),
		MovementDate: mv.Date, DocumentID: uuid.UUID(mv.DocumentID), LineNo: mv.LineNo, Quantity: mv.Quantity, Value: mv.Value,
	}
}

func MovementToDomain(m models.MovementModel) services.Movement {
	return services.Movement{
		ID: vo.MovementID(m.ID), Key: services.StockKey{ItemID: vo.ItemID(m.ItemID), WarehouseID: vo.WarehouseID(m.WarehouseID)},
		Date: DateUTC(m.MovementDate), DocumentID: vo.DocumentID(m.DocumentID), LineNo: m.LineNo, Quantity: m.Quantity, Value: m.Value,
	}
}
