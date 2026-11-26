// Package gormpersistence คือการแปลงระหว่าง entity ของ manufacturing กับ gorm model (ชัดเจนทุก field) + migration / transaction
package gormpersistence

import (
	"github.com/google/uuid"

	"apexis/backend/internal/modules/manufacturing/domain/entities"
	vo "apexis/backend/internal/modules/manufacturing/domain/value_objects"
	"apexis/backend/internal/modules/manufacturing/infrastructure/persistence/gorm/models"
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

// ---------- ค่าตั้ง ----------

func SettingsToModel(companyID vo.CompanyID, s entities.Settings) models.SettingsModel {
	return models.SettingsModel{CompanyID: uuid.UUID(companyID), JournalID: optUUID(s.JournalID), WipAccountID: optUUID(s.WipAccountID),
		ConversionAccountID: optUUID(s.ConversionAccountID), DefaultWarehouseID: optUUID(s.DefaultWarehouseID)}
}

func SettingsToEntity(m models.SettingsModel) entities.Settings {
	return entities.Settings{JournalID: optID[vo.JournalID](m.JournalID), WipAccountID: optID[vo.AccountID](m.WipAccountID),
		ConversionAccountID: optID[vo.AccountID](m.ConversionAccountID), DefaultWarehouseID: optID[vo.WarehouseID](m.DefaultWarehouseID)}
}

// ---------- สูตร ----------

func BomToModels(b *entities.Bom) (models.BomModel, []models.BomComponentModel, []models.BomConversionModel) {
	d := b.Data()
	created, updated := b.Times()
	m := models.BomModel{ID: uuid.UUID(b.ID()), CompanyID: uuid.UUID(b.CompanyID()), Code: d.Code, Name: d.Name, ItemID: uuid.UUID(d.ItemID),
		OutputQuantity: d.OutputQuantity, Status: string(d.Status), IsDefault: d.IsDefault, CreatedAt: created, UpdatedAt: updated}
	components := make([]models.BomComponentModel, len(b.Components()))
	for i, c := range b.Components() {
		components[i] = models.BomComponentModel{BomID: m.ID, LineNo: c.LineNo, ItemID: uuid.UUID(c.ItemID), Quantity: c.Quantity, ScrapPercent: c.ScrapPercent}
	}
	conversions := make([]models.BomConversionModel, len(b.Conversions()))
	for i, c := range b.Conversions() {
		conversions[i] = models.BomConversionModel{BomID: m.ID, LineNo: c.LineNo, Description: c.Description, AccountID: optUUID(c.AccountID), Amount: c.Amount}
	}
	return m, components, conversions
}

func BomToEntity(m models.BomModel, components []models.BomComponentModel, conversions []models.BomConversionModel) *entities.Bom {
	cs := make([]entities.Component, len(components))
	for i, c := range components {
		cs[i] = entities.Component{LineNo: c.LineNo, ComponentData: entities.ComponentData{ItemID: vo.ItemID(c.ItemID), Quantity: c.Quantity, ScrapPercent: c.ScrapPercent}}
	}
	vs := make([]entities.Conversion, len(conversions))
	for i, c := range conversions {
		vs[i] = entities.Conversion{LineNo: c.LineNo, ConversionData: entities.ConversionData{Description: c.Description,
			AccountID: optID[vo.AccountID](c.AccountID), Amount: c.Amount}}
	}
	d := entities.BomData{Code: m.Code, Name: m.Name, ItemID: vo.ItemID(m.ItemID), OutputQuantity: m.OutputQuantity,
		Status: vo.Status(m.Status), IsDefault: m.IsDefault}
	return entities.RehydrateBom(vo.BomID(m.ID), vo.CompanyID(m.CompanyID), d, cs, vs, m.CreatedAt, m.UpdatedAt)
}

// ---------- ใบสั่งผลิต ----------

type OrderModels struct {
	Order       models.OrderModel
	Materials   []models.OrderMaterialModel
	Conversions []models.OrderConversionModel
	Movements   []models.MovementModel
}

func OrderToModels(o *entities.Order) OrderModels {
	s := o.State()
	d := s.Data
	out := OrderModels{Order: models.OrderModel{
		ID: uuid.UUID(s.ID), CompanyID: uuid.UUID(s.CompanyID), DocumentNo: s.DocumentNo, OrderDate: d.Date, ItemID: uuid.UUID(d.ItemID),
		BomID: uuid.UUID(d.BomID), Quantity: d.Quantity, OutputQuantity: s.OutputQuantity, WarehouseID: uuid.UUID(d.WarehouseID),
		PlannedStart: d.PlannedStart, PlannedEnd: d.PlannedEnd, Status: string(s.Status), Description: d.Description,
		CompletedQuantity: s.CompletedQuantity, CompletedAt: s.CompletedAt, ConversionApplied: s.ConversionApplied,
		CancelledAt: s.CancelledAt, CancelReason: s.CancelReason, CreatedBy: uuid.UUID(s.CreatedBy), CreatedAt: s.CreatedAt,
	}}
	for _, m := range s.Materials {
		out.Materials = append(out.Materials, models.OrderMaterialModel{OrderID: uuid.UUID(s.ID), LineNo: m.LineNo, ItemID: uuid.UUID(m.ItemID),
			Required: m.Required, Issued: m.Issued, Returned: m.Returned, IssuedCost: m.IssuedCost, ReturnedCost: m.ReturnedCost})
	}
	for _, c := range s.Conversions {
		out.Conversions = append(out.Conversions, models.OrderConversionModel{OrderID: uuid.UUID(s.ID), LineNo: c.LineNo,
			Description: c.Description, AccountID: optUUID(c.AccountID), BatchAmount: c.BatchAmount})
	}
	for i, mv := range s.Movements {
		out.Movements = append(out.Movements, models.MovementModel{ID: uuid.UUID(mv.ID), OrderID: uuid.UUID(s.ID), Position: i + 1,
			Kind: string(mv.Kind), MovementDate: mv.Date, StockDocumentID: uuid.UUID(mv.StockDocumentID), StockDocumentNo: mv.StockDocumentNo,
			Value: mv.Value, EntryID: optUUID(mv.EntryID), EntryNo: mv.EntryNo, Cancelled: mv.Cancelled})
	}
	return out
}

func OrderToEntity(m OrderModels) *entities.Order {
	o := m.Order
	s := entities.OrderState{
		ID: vo.OrderID(o.ID), CompanyID: vo.CompanyID(o.CompanyID), DocumentNo: o.DocumentNo,
		Data: entities.OrderData{Date: o.OrderDate, ItemID: vo.ItemID(o.ItemID), BomID: vo.BomID(o.BomID), Quantity: o.Quantity,
			WarehouseID: vo.WarehouseID(o.WarehouseID), PlannedStart: o.PlannedStart, PlannedEnd: o.PlannedEnd, Description: o.Description},
		OutputQuantity: o.OutputQuantity, Status: vo.OrderStatus(o.Status), CompletedQuantity: o.CompletedQuantity, CompletedAt: o.CompletedAt,
		ConversionApplied: o.ConversionApplied, CancelledAt: o.CancelledAt, CancelReason: o.CancelReason, CreatedBy: vo.UserID(o.CreatedBy),
		CreatedAt: o.CreatedAt,
	}
	for _, x := range m.Materials {
		s.Materials = append(s.Materials, entities.Material{LineNo: x.LineNo, ItemID: vo.ItemID(x.ItemID), Required: x.Required,
			Issued: x.Issued, Returned: x.Returned, IssuedCost: x.IssuedCost, ReturnedCost: x.ReturnedCost})
	}
	for _, x := range m.Conversions {
		s.Conversions = append(s.Conversions, entities.OrderConversion{LineNo: x.LineNo, Description: x.Description,
			AccountID: optID[vo.AccountID](x.AccountID), BatchAmount: x.BatchAmount})
	}
	for _, x := range m.Movements {
		s.Movements = append(s.Movements, entities.Movement{ID: vo.MovementID(x.ID), Kind: vo.MovementKind(x.Kind), Date: x.MovementDate,
			StockDocumentID: vo.StockDocumentID(x.StockDocumentID), StockDocumentNo: x.StockDocumentNo, Value: x.Value,
			EntryID: optID[vo.EntryID](x.EntryID), EntryNo: x.EntryNo, Cancelled: x.Cancelled})
	}
	return entities.RehydrateOrder(s)
}
