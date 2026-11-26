package entities

import (
	"slices"

	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
)

var (
	goodsTypes = []vo.BusinessType{vo.BusinessTrading, vo.BusinessManufacturing}
	mfgTypes   = []vo.BusinessType{vo.BusinessManufacturing}
	conTypes   = []vo.BusinessType{vo.BusinessConstruction}
	// ก่อสร้างรับวัสดุเข้าก่อนได้ใบแจ้งหนี้ได้เช่นเดียวกับธุรกิจที่มีสินค้า
	receivingTypes = []vo.BusinessType{vo.BusinessTrading, vo.BusinessManufacturing, vo.BusinessConstruction}
)

// ChartScope คือบัญชีของผังมาตรฐานที่สร้างให้เฉพาะบางประเภทธุรกิจ — รหัสที่ไม่อยู่ในแผนที่นี้ = ทุกประเภท
//
// ก่อสร้างไม่ได้หมวดขายสินค้า / ต้นทุนขาย (4050 / 5050): รายได้คือรายได้จากสัญญา และการซื้อวัสดุลงต้นทุนงานก่อสร้าง
// (ค่าวัสดุก่อสร้าง 5071000000) หรือวัสดุก่อสร้างคงเหลือ (1105100000) โดยตรง
var ChartScope = map[string][]vo.BusinessType{
	// สินค้าคงเหลือ
	"1105040000": goodsTypes, // สินค้าสำเร็จรูป / สินค้าเพื่อขาย
	"1105050000": mfgTypes,   // งานระหว่างผลิต
	"1105060000": mfgTypes,   // วัตถุดิบ
	"1105070000": mfgTypes,   // วัสดุโรงงานและวัสดุสิ้นเปลือง
	"1105080000": goodsTypes, // สินค้าระหว่างทาง
	"1105090000": goodsTypes, // ค่าเผื่อการลดมูลค่าสินค้าคงเหลือ
	"2102030000": receivingTypes,
	// รายได้จากการขาย / ต้นทุนขาย
	"4050000000": goodsTypes, "4051000000": goodsTypes, "4052000000": goodsTypes,
	"5050000000": goodsTypes, "5051000000": goodsTypes, "5052000000": goodsTypes, "5053000000": goodsTypes,
	"5054000000": goodsTypes, "5055000000": goodsTypes, "5056000000": goodsTypes,
	// ต้นทุนการผลิต
	"5060000000": mfgTypes, "5061000000": mfgTypes, "5062000000": mfgTypes, "5063000000": mfgTypes,
	"5063010000": mfgTypes, "5063020000": mfgTypes, "5063030000": mfgTypes, "5063040000": mfgTypes,
	"5063050000": mfgTypes, "5063060000": mfgTypes, "5064000000": mfgTypes,
	// สัญญาก่อสร้าง
	"1103040000": conTypes, "1103050000": conTypes, "1105100000": conTypes, "2103040000": conTypes,
	"4060000000": conTypes, "4061000000": conTypes, "4062000000": conTypes,
	"5070000000": conTypes, "5071000000": conTypes, "5072000000": conTypes, "5073000000": conTypes,
	"5074000000": conTypes, "5075000000": conTypes, "5076000000": conTypes, "5077000000": conTypes,
}

// InChart: บัญชีมาตรฐานรหัสนี้อยู่ในผังของประเภทธุรกิจ bt หรือไม่
func InChart(code string, bt vo.BusinessType) bool {
	types, scoped := ChartScope[code]
	return !scoped || slices.Contains(types, bt)
}

// ChartFor คือบัญชีมาตรฐานของประเภทธุรกิจ bt (คงลำดับบัญชีคุมก่อนบัญชีย่อย)
func ChartFor(bt vo.BusinessType) []DefaultAccount {
	out := make([]DefaultAccount, 0, len(DefaultChart))
	for _, d := range DefaultChart {
		if InChart(d.Code, bt) {
			out = append(out, d)
		}
	}
	return out
}
