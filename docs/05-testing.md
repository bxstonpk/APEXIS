# 5. Testing

## 5.1 กลยุทธ์

| ระดับ | ทดสอบอะไร | ต้องมี DB |
|---|---|---|
| Unit — domain | entity invariant, value object, domain service (pure function) | ไม่ |
| Unit — platform | parser ของ URL query, escape `LIKE` | ไม่ |
| Integration (ต่อ module) | ยิง HTTP จริงผ่าน `httptest` → use case → GORM → PostgreSQL จริง ครอบคลุม flow, สิทธิ์, tenant isolation, error code | ใช่ |

integration test **ข้ามอัตโนมัติ** ถ้าไม่ได้ตั้ง `IDENTITY_TEST_DSN` (ทั้งสอง module ใช้ตัวแปรเดียวกัน)
test จะ drop / สร้างตารางใหม่ — **ห้ามชี้ไปที่ฐานข้อมูลจริง**

## 5.2 วิธีรัน

```bash
cd apps/backend

# unit test อย่างเดียว
go vet ./...
go test ./...

# integration test — ใช้ Postgres ชั่วคราวใน Docker
docker run -d --rm --name apexis-test-pg -e POSTGRES_PASSWORD=test -e POSTGRES_DB=apexis_test \
  -p 55432:5432 postgres:16-alpine

IDENTITY_TEST_DSN="postgres://postgres:test@localhost:55432/apexis_test?sslmode=disable" \
  go test -p 1 ./...        # -p 1: สอง package drop ตารางชุดเดียวกัน ห้ามรันขนาน

docker stop apexis-test-pg
```

PowerShell: `$env:IDENTITY_TEST_DSN = "postgres://…"; go test -p 1 ./...`

## 5.3 รายการ test ที่มี

### identity

| ไฟล์ | Test | ครอบคลุม |
|---|---|---|
| `domain/services/access_test.go` | `TestResolveAccess*` (4), `TestMembershipChangeStatus`, `TestMembershipRejectsRoleFromOtherCompany`, `TestOwnerRoleIsImmutable` | รวมสิทธิ์จากหลาย role, สถานะไม่ active ถูกปฏิเสธ, role ต่างบริษัท, owner แก้ไม่ได้ |
| `domain/value_objects/permission_test.go` | `TestParsePermission`, `TestPermissionCovers`, `TestPermissionSetAllowsAll` | รูปแบบสิทธิ์และ wildcard |
| `identity_integration_test.go` | `TestMultiCompanyAccessFlow` | สมัคร → ล็อกอิน → ตั้ง owner ของบริษัท (`ProvisionCompanyOwner`) → role → เพิ่มสมาชิก → ระงับ → สิทธิ์หลายบริษัท, timestamp จัดการโดย GORM |
| | `TestListSearchViaURLQuery` | ค้นหา / กรอง / เรียง / แบ่งหน้าสมาชิกและ role ผ่าน URL query, ค่าผิดได้ 400 |

### organization

| ไฟล์ | Test | ครอบคลุม |
|---|---|---|
| `domain/value_objects/values_test.go` | `TestNewCode`, `TestTaxBranchNo`, `TestNewAddress`, `TestEnsureDateRange` | รูปแบบรหัส, เลขสาขา, ที่อยู่/รหัสไปรษณีย์, ช่วงวันที่ |
| `domain/entities/entities_test.go` | `TestHeadOfficeRules`, `TestHierarchyCycle`, `TestCostCenterValidity`, `TestClosedProjectNeedsEndDate`, `TestFiscalYearPeriods`, `TestFiscalPeriodCloseOrder`, `TestCompanyRules` | กฎธุรกิจทั้งหมดของ entity (รวมค่าเริ่มต้นของบริษัท, รหัสบริษัทแก้ไม่ได้) |
| `organization_integration_test.go` | `TestOrganizationFlow` | สร้างบริษัท (ค่าเริ่มต้น, รหัสซ้ำ), **rollback ข้าม module เมื่อ identity ล้มเหลว**, แก้บริษัท, บริษัทของฉัน, บริษัท inactive = อ่านอย่างเดียว / เปิดกลับ, สำนักงานใหญ่, วนลูป BU, การอ้างอิง / ลบไม่ได้เพราะมีผู้ใช้, `active_on`, ค้นหาช่วงวันที่โครงการ, soft delete + ใช้รหัสซ้ำ, ปีบัญชีซ้อน, ลำดับปิด/เปิดงวด, สิทธิ์ (พนักงาน / คนนอก / ไม่มี token), tenant isolation, query ผิดรูปแบบ |

### ledger

| ไฟล์ | Test | ครอบคลุม |
|---|---|---|
| `domain/value_objects/money_test.go` | `TestParseMoney` | แปลงจำนวนเงินเป็น decimal, ขอบเขต NUMERIC(24,6) |
| `domain/entities/entities_test.go` | `TestAccountParentRules`, `TestAccountUsageRules`, `TestEntryLineValidation`, `TestEntryPostAndReverse`, `TestEntryAmountDecimalsFromCompany`, `TestEntryNeedsTwoLines` | กฎผังบัญชี, บรรทัดรายการ, draft → posted → reversed |
| `domain/services/services_test.go` | `TestBuildSectionRollsUpAndHidesZero` | รวมยอดตามผังบัญชีในงบการเงิน |
| `ledger_integration_test.go` | `TestLedgerFlow` | สมุด / ผังบัญชี (ลูป, parent, ซ้ำ), draft ไม่สมดุล → แก้ → ผ่าน, เลขที่เอกสารรายปี, มิติวิเคราะห์ไม่พบ, บัญชีคุม / inactive, งวดปิด, ไม่มีงวด, กลับรายการ, ค้นหาผ่าน URL query, **ตัวเลขงบทดลอง / งบแสดงฐานะ / งบกำไรขาดทุน / บัญชีแยกประเภท (คำนวณมือแล้วเทียบ)**, ลบที่ถูกใช้, **ผ่านรายการพร้อมกัน 10 ใบ ได้เลขที่ไม่ซ้ำไม่ขาดตอน**, สิทธิ์ (draft ได้แต่ post ไม่ได้), บริษัท inactive, tenant isolation, query ผิดรูปแบบ |

### inventory

| ไฟล์ | Test | ครอบคลุม |
|---|---|---|
| `domain/services/fifo_test.go` | `TestReceiptValueRoundsToCompanyDecimals`, `TestIssueConsumesOldestLayersFirst`, `TestPartialConsumptionLeavesNoRoundingDust`, `TestNoNegativeStockAndNoFutureLayers`, `TestTransferCarriesFifoCost`, `TestAdjustmentInThenOutInSameDocument`, `TestCancelRestoresLayersAndRejectsUsedReceipt` | FIFO (pure function ไม่ต้องใช้ DB) |
| `domain/entities/entities_test.go` | `TestDocumentLineRules`, `TestDocumentLifecycle`, `TestItemRules` | กฎบรรทัดเอกสาร, ทศนิยมตามบริษัท, draft → posted → cancelled |
| `domain/services/fifo_test.go` | `TestPlanClosingAdjustsInventoryAgainstCogs` | รายการปรับปรุงสิ้นงวด (ปรับสินค้าคงเหลือ, ส่วนต่างลงต้นทุนขาย) |
| `domain/entities/entities_test.go` | `TestAccountingSystemByFiscalYear` | ระบบบัญชีตามปี, ล็อกปีที่มีเอกสาร |
| `closing_integration_test.go` | `TestPeriodicClosing` | บัญชีของหมวด / สินค้าตรวจกับ ledger, ค่าตั้ง, ระบบบัญชีต่อปี, **ปิดงวด ม.ค. / ก.พ. ส่วนต่างคำนวณมือแล้วเทียบ**, ห้ามปิดซ้ำ, ห้ามเอกสารย้อนหลังในงวดที่ปิด, ยกเลิกย้อนจากงวดล่าสุด |
| `inventory_integration_test.go` | `TestInventoryFlow` | master / สาขาไม่พบ, รับ / เบิก / โอน / ปรับปรุง, **มูลค่า FIFO คำนวณมือแล้วเทียบ**, ห้ามติดลบ / ห้ามใช้ชั้นวันที่อนาคต, ทศนิยมตามบริษัท, รายงาน stock-on-hand / stock-card (แบ่งหน้า), ลำดับการยกเลิก, **เบิกพร้อมกัน 10 ครั้งจากของ 5 ชิ้น = สำเร็จ 5 พอดี**, สิทธิ์, งวดปิด, บริษัท inactive, query ผิดรูปแบบ |

### purchasing

| ไฟล์ | Test | ครอบคลุม |
|---|---|---|
| `domain/entities/entities_test.go` | `TestVatTermsFollowSupplierAtCreation`, `TestLineAmounts`, `TestOrderReceiveInvoiceAndAutoClose`, `TestInvoiceJournalBalances` | VAT ตามผู้ขาย, รวม / แยก VAT, รับไม่เกิน / แจ้งหนี้ไม่เกิน, ล้าง GR/IR ไม่มีเศษปัด, ปิดอัตโนมัติ, รายการบัญชีสมดุล |
| `purchasing_integration_test.go` | `TestPurchasingFlow` | ค่าตั้ง / ผู้ขาย, ใบสั่งซื้อ VAT, สิทธิ์อนุมัติ, ผู้ขายเลิกจด VAT แล้วใบเดิมยังคิด VAT, **รับสินค้า → สต็อก FIFO + GL, แจ้งหนี้ราคาต่าง → ผลต่างราคา, งบทดลองเทียบตัวเลข**, รายงานค้างรับ / อายุเจ้าหนี้, ลำดับการยกเลิก (GL / สต็อกกลับเป็นศูนย์), เลขใบแจ้งหนี้ซ้ำ, ปิดใบสั่งซื้อ, **periodic: รับสินค้าลงบัญชีซื้อ**, ปิดอัตโนมัติ |

| `domain/entities/entities_test.go` | `TestWithholdingOnPreVatAmount`, `TestPaymentRulesAndJournalOneLinePerAccount`, `TestInvoicePayRules` | ฐานภาษีหัก ณ ที่จ่าย (จ่ายครบ / บางส่วน), ยอดช่องทางจ่าย, **รายการบัญชี 1 ผังบัญชี = 1 บรรทัด**, จ่ายเกิน / ผู้ขายผิด, ใบแจ้งหนี้ที่จ่ายแล้วยกเลิกไม่ได้ |
| `payment_integration_test.go` | `TestSupplierPayments` | **รูปแบบเลขที่ (organization + สมุดรายวัน พ.ศ.)**, ทะเบียนเงินสด / ธนาคาร, ประเภทภาษีหัก ณ ที่จ่าย, จ่าย 2 ใบแจ้งหนี้ / 2 ช่องทาง, **บรรทัดบัญชีเทียบตัวเลข**, ยอดค้าง, อายุเจ้าหนี้ก่อน / หลังจ่าย, รายงาน ภ.ง.ด.53 / 3, ยกเลิกแล้วยอดกลับ, เลขที่ต่อเนื่อง |

### platform

| ไฟล์ | Test |
|---|---|
| `platform/numeric/numeric_test.go` | `TestParse`, `TestFitsScale`, `TestInRange` |
| `platform/numbering/numbering_test.go` | `TestRenderAndPeriodKey`, `TestNormalizeRejects` |
| `platform/listing/listing_test.go` | `TestParsePage`, `TestParseSort`, `TestParseSearch`, `TestParseDateRange`, `TestParseListAndOptional` |
| `platform/database/database_test.go` | `TestLikeContainsEscapesWildcards` |

## 5.4 ผลล่าสุด (2026-11-21)

`go build ./...`, `go vet ./...` และ `go test -p 1 ./...` (รวม integration กับ PostgreSQL 16) — **ผ่านทั้งหมด**

## 5.5 Checklist ก่อนถือว่า API เสร็จ

ใช้ checklist ใน [CLAUDE.md §32](../CLAUDE.md) — architecture, GET (N+1 / JOIN / pagination), CREATE / UPDATE / DELETE (transaction), GORM, security

## 5.6 สิ่งที่ยังไม่มี

- CI ที่รัน test อัตโนมัติ
- test วัดจำนวนคิวรีต่อ request แบบอัตโนมัติ (ตอนนี้ตรวจด้วยมือจาก log ของ Postgres)
- load / performance test
- frontend test
