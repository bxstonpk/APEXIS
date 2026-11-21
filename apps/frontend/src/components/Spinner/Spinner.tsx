/** วงหมุนแสดงสถานะกำลังทำงาน — ตกแต่งอย่างเดียว ข้อความสถานะให้ผู้ใช้ component แสดงเอง */
export function Spinner() {
  return (
    <span
      aria-hidden="true"
      className="inline-block size-4 animate-spin rounded-full border-2 border-current border-r-transparent motion-reduce:[animation-duration:2s]"
    />
  )
}
