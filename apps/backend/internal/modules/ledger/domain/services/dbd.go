package services

import (
	"sort"
	"strings"

	"github.com/shopspring/decimal"

	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
)

// งบการเงินตามแบบที่กรมพัฒนาธุรกิจการค้ากำหนด (กิจการที่ไม่ใช่บริษัทจดทะเบียน)
//
// จัดบัญชีเข้าบรรทัดของงบด้วยรหัสขึ้นต้น (prefix) ของรหัสบัญชีตามโครงสร้างผังบัญชีมาตรฐาน
// (ทั้ง 4 ประเภทธุรกิจ) — prefix ที่ยาวที่สุดที่ตรงชนะ; บัญชีที่ไม่ตรง prefix ใดลงบรรทัด "อื่น" ของหมวด
// จึงรวมยอดได้ตรงกับงบทดลองเสมอ (บัญชีพักแสดงในสินทรัพย์หมุนเวียนอื่น — ด้านเดบิตเป็นบวก)
//
// key ของบรรทัดเป็นรหัสคงที่ หน้าเว็บแปลเป็นชื่อบรรทัดเอง

// DBDRowKind คือชนิดบรรทัด: header = หัวข้อ (ไม่มียอด), line = บรรทัดยอด, subtotal / total = ยอดรวม
type DBDRowKind string

const (
	DBDHeader   DBDRowKind = "header"
	DBDLine     DBDRowKind = "line"
	DBDSubtotal DBDRowKind = "subtotal"
	DBDTotal    DBDRowKind = "total"
)

// DBDRow คือบรรทัดของงบ — Amounts มี 1 ค่าต่อคอลัมน์ (ปีปัจจุบัน, ปีก่อน)
type DBDRow struct {
	Key     string
	Kind    DBDRowKind
	Level   int
	Amounts []vo.Money
}

type dbdMap struct {
	prefix string
	key    string
}

// bsMap / isMap: prefix -> บรรทัด (ผังมาตรฐานทุกประเภทธุรกิจ รวมผลิต 5060 / ก่อสร้าง 4060, 5070)
var bsMap = []dbdMap{
	{"1101", "cash"}, {"1102", "short_investments"}, {"1103", "receivables"}, {"1104", "short_loans"},
	{"1105", "inventories"}, {"1106", "other_current_assets"}, {"1107", "other_current_assets"},
	{"1201", "long_investments"}, {"1202", "long_investments"}, {"1203", "long_receivables"}, {"1204", "long_receivables"},
	{"1205", "investment_property"}, {"1206", "ppe"}, {"1207", "intangibles"}, {"1208", "intangibles"},
	{"1209", "other_noncurrent_assets"}, {"1210", "other_noncurrent_assets"},
	{"2101", "short_borrowings"}, {"2106", "short_borrowings"}, {"2102", "payables"},
	{"2104", "current_long_term"}, {"2105", "current_long_term"}, {"2107", "income_tax_payable"},
	{"2103", "other_current_liabilities"}, {"2108", "other_current_liabilities"}, {"2109", "other_current_liabilities"},
	{"2201", "long_borrowings"}, {"2202", "lease_liabilities"},
	{"2203", "other_noncurrent_liabilities"}, {"2204", "other_noncurrent_liabilities"}, {"2205", "other_noncurrent_liabilities"}, {"2206", "other_noncurrent_liabilities"},
	{"31", "share_capital"}, {"32", "share_premium"}, {"3301", "legal_reserve"},
	{"3302", "retained_unappropriated"}, {"3303", "retained_unappropriated"}, {"3304", "other_equity"},
}

// รายได้ / ค่าใช้จ่าย: กลุ่มระดับ 2 ของผังมาตรฐาน — 405x / 406x / 41xx / 42xx, 505x / 506x / 507x / 51xx ... 55xx
var isMap = []dbdMap{
	{"405", "revenue_sales"}, {"406", "revenue_construction"}, {"41", "revenue_services"}, {"42", "other_income"},
	{"505", "cost_of_sales"}, {"506", "cost_of_sales"}, {"507", "cost_of_construction"}, {"51", "cost_of_services"},
	{"52", "selling_expenses"}, {"53", "admin_expenses"}, {"54", "finance_costs"}, {"55", "income_tax"},
}

func init() {
	for _, m := range [][]dbdMap{bsMap, isMap} {
		sort.SliceStable(m, func(i, j int) bool { return len(m[i].prefix) > len(m[j].prefix) })
	}
}

// DBDLineKey คืนบรรทัดของงบที่บัญชีนี้อยู่ ("" = ไม่อยู่ในงบนี้)
func DBDLineKey(a AccountNode) string {
	switch a.Type {
	case vo.AccountAsset, vo.AccountLiability, vo.AccountEquity, vo.AccountSuspense:
		if a.Type == vo.AccountSuspense {
			return "other_current_assets"
		}
		if k := match(bsMap, a.Code); k != "" {
			return k
		}
		switch a.Type {
		case vo.AccountAsset:
			if strings.HasPrefix(a.Code, "11") {
				return "other_current_assets"
			}
			return "other_noncurrent_assets"
		case vo.AccountLiability:
			if strings.HasPrefix(a.Code, "21") {
				return "other_current_liabilities"
			}
			return "other_noncurrent_liabilities"
		}
		return "other_equity"
	default:
		if k := match(isMap, a.Code); k != "" {
			return k
		}
		if a.Type == vo.AccountRevenue {
			return "other_income"
		}
		return "admin_expenses"
	}
}

func match(m []dbdMap, code string) string {
	for _, x := range m {
		if strings.HasPrefix(code, x.prefix) {
			return x.key
		}
	}
	return ""
}

// BSColumn คือข้อมูลหนึ่งคอลัมน์ของงบแสดงฐานะการเงิน
//
//	Net: ยอดสะสม (เดบิต - เครดิต) ของบัญชีงบแสดงฐานะ + บัญชีพัก ณ วันที่
//	PriorUnclosed: กำไร (ขาดทุน) ของปีก่อนที่ยังไม่ปิดบัญชี (เครดิตเป็นบวก)
//	CurrentYear: กำไร (ขาดทุน) ปีปัจจุบันถึงวันที่ (เครดิตเป็นบวก)
type BSColumn struct {
	Net           map[vo.AccountID]vo.Money
	PriorUnclosed vo.Money
	CurrentYear   vo.Money
}

// ISColumn: ยอด (เดบิต - เครดิต) ของบัญชีรายได้ / ค่าใช้จ่ายในช่วง (ไม่รวมรายการปิดบัญชี)
type ISColumn map[vo.AccountID]vo.Money

// DBDBalanceSheet สร้างงบแสดงฐานะการเงิน — สินทรัพย์ (+ บัญชีพัก) = หนี้สิน + ส่วนของผู้ถือหุ้น เสมอ
func DBDBalanceSheet(chart []AccountNode, cols []BSColumn) []DBDRow {
	n := len(cols)
	sums := make([]map[string]vo.Money, n)
	for i, c := range cols {
		sums[i] = map[string]vo.Money{}
		for _, a := range chart {
			v, ok := c.Net[a.ID]
			if !ok || !a.Postable && v.IsZero() {
				continue
			}
			k := DBDLineKey(a)
			if !a.Type.DebitNormal() {
				v = v.Neg()
			}
			sums[i][k] = sums[i][k].Add(v)
		}
		sums[i]["retained_unclosed"] = c.PriorUnclosed
		sums[i]["current_year_profit"] = c.CurrentYear
	}
	b := newBuilder(n, sums)
	b.header("assets", 0)
	b.header("current_assets", 1)
	ca := b.lines(2, "cash", "short_investments", "receivables", "short_loans", "inventories", "other_current_assets")
	b.subtotal("total_current_assets", 1, ca)
	b.header("noncurrent_assets", 1)
	nca := b.lines(2, "long_investments", "long_receivables", "investment_property", "ppe", "intangibles", "other_noncurrent_assets")
	b.subtotal("total_noncurrent_assets", 1, nca)
	b.total("total_assets", 0, add(ca, nca))
	b.header("liabilities_equity", 0)
	b.header("current_liabilities", 1)
	cl := b.lines(2, "short_borrowings", "payables", "current_long_term", "income_tax_payable", "other_current_liabilities")
	b.subtotal("total_current_liabilities", 1, cl)
	b.header("noncurrent_liabilities", 1)
	ncl := b.lines(2, "long_borrowings", "lease_liabilities", "other_noncurrent_liabilities")
	b.subtotal("total_noncurrent_liabilities", 1, ncl)
	liab := add(cl, ncl)
	b.subtotal("total_liabilities", 1, liab)
	b.header("equity", 1)
	eq := b.lines(2, "share_capital", "share_premium", "legal_reserve", "retained_unappropriated", "retained_unclosed", "current_year_profit", "other_equity")
	b.subtotal("total_equity", 1, eq)
	b.total("total_liabilities_equity", 0, add(liab, eq))
	return b.rows
}

// DBDIncomeStatement สร้างงบกำไรขาดทุน — กำไรสุทธิ = -(ผลรวมเดบิต - เครดิตของรายได้ / ค่าใช้จ่าย) ในช่วงเสมอ
func DBDIncomeStatement(chart []AccountNode, cols []ISColumn) []DBDRow {
	n := len(cols)
	sums := make([]map[string]vo.Money, n)
	for i, c := range cols {
		sums[i] = map[string]vo.Money{}
		for _, a := range chart {
			if a.Type != vo.AccountRevenue && a.Type != vo.AccountExpense {
				continue
			}
			v, ok := c[a.ID]
			if !ok {
				continue
			}
			if a.Type == vo.AccountRevenue {
				v = v.Neg()
			}
			k := DBDLineKey(a)
			sums[i][k] = sums[i][k].Add(v)
		}
	}
	b := newBuilder(n, sums)
	b.header("revenue", 0)
	rev := b.lines(1, "revenue_sales", "revenue_services", "revenue_construction")
	b.subtotal("total_revenue", 0, rev)
	b.header("cost", 0)
	cost := b.lines(1, "cost_of_sales", "cost_of_services", "cost_of_construction")
	b.subtotal("total_cost", 0, cost)
	gross := sub(rev, cost)
	b.subtotal("gross_profit", 0, gross)
	other := b.lines(0, "other_income")
	opex := b.lines(0, "selling_expenses", "admin_expenses")
	ebit := sub(add(gross, other), opex)
	b.subtotal("profit_before_finance_costs", 0, ebit)
	fin := b.lines(0, "finance_costs")
	pbt := sub(ebit, fin)
	b.subtotal("profit_before_tax", 0, pbt)
	tax := b.lines(0, "income_tax")
	b.total("net_profit", 0, sub(pbt, tax))
	return b.rows
}

type builder struct {
	n    int
	sums []map[string]vo.Money
	rows []DBDRow
}

func newBuilder(n int, sums []map[string]vo.Money) *builder { return &builder{n: n, sums: sums} }

func (b *builder) header(key string, level int) {
	b.rows = append(b.rows, DBDRow{Key: key, Kind: DBDHeader, Level: level, Amounts: []vo.Money{}})
}

// lines เพิ่มบรรทัดยอด (แสดงทุกบรรทัดแม้เป็นศูนย์ — โครงงบคงที่) และคืนผลรวม
func (b *builder) lines(level int, keys ...string) []vo.Money {
	total := zeros(b.n)
	for _, k := range keys {
		am := make([]vo.Money, b.n)
		for i := range am {
			am[i] = b.sums[i][k]
			total[i] = total[i].Add(am[i])
		}
		b.rows = append(b.rows, DBDRow{Key: k, Kind: DBDLine, Level: level, Amounts: am})
	}
	return total
}

func (b *builder) subtotal(key string, level int, v []vo.Money) {
	b.rows = append(b.rows, DBDRow{Key: key, Kind: DBDSubtotal, Level: level, Amounts: v})
}

func (b *builder) total(key string, level int, v []vo.Money) {
	b.rows = append(b.rows, DBDRow{Key: key, Kind: DBDTotal, Level: level, Amounts: v})
}

func zeros(n int) []vo.Money {
	out := make([]vo.Money, n)
	for i := range out {
		out[i] = decimal.Zero
	}
	return out
}

func add(a, b []vo.Money) []vo.Money {
	out := make([]vo.Money, len(a))
	for i := range a {
		out[i] = a[i].Add(b[i])
	}
	return out
}

func sub(a, b []vo.Money) []vo.Money {
	out := make([]vo.Money, len(a))
	for i := range a {
		out[i] = a[i].Sub(b[i])
	}
	return out
}

// DBDValue คืนยอดของบรรทัด key ในคอลัมน์ col (ใช้ในเทสต์ / ตรวจยอด)
func DBDValue(rows []DBDRow, key string, col int) vo.Money {
	for _, r := range rows {
		if r.Key == key && col < len(r.Amounts) {
			return r.Amounts[col]
		}
	}
	return decimal.Zero
}
