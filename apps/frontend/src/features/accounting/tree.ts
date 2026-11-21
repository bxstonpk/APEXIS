import type { Account } from './types'

/**
 * ผังบัญชีแบบต้นไม้ (pure) — สร้างจาก parent ของ GET /accounts/tree (เรียงตามรหัสแล้ว)
 * บัญชีที่ parent หาไม่พบ (เช่น parent ถูกลบ) ถือเป็นระดับบนสุด จึงไม่หายจากผัง
 */
export type TreeNode = { account: Account; depth: number; children: TreeNode[] }

export function buildTree(accounts: readonly Account[]): TreeNode[] {
  const nodes = new Map<string, TreeNode>()
  for (const a of accounts) nodes.set(a.id, { account: a, depth: 0, children: [] })
  const roots: TreeNode[] = []
  for (const a of accounts) {
    const node = nodes.get(a.id)!
    const parent = a.parent && nodes.get(a.parent.id)
    if (parent && parent !== node) parent.children.push(node)
    else roots.push(node)
  }
  const byCode = (x: TreeNode, y: TreeNode) => (x.account.code < y.account.code ? -1 : x.account.code > y.account.code ? 1 : 0)
  const setDepth = (list: TreeNode[], depth: number, seen: Set<string>) => {
    list.sort(byCode)
    for (const n of list) {
      if (seen.has(n.account.id)) continue
      seen.add(n.account.id)
      n.depth = depth
      setDepth(n.children, depth + 1, seen)
    }
  }
  setDepth(roots, 0, new Set())
  return roots
}

/** แถวที่มองเห็นตามลำดับ (ลูกของ node ที่ยุบอยู่ไม่แสดง) */
export function visibleRows(roots: readonly TreeNode[], expanded: ReadonlySet<string>): TreeNode[] {
  const out: TreeNode[] = []
  const walk = (list: readonly TreeNode[]) => {
    for (const n of list) {
      out.push(n)
      if (n.children.length && expanded.has(n.account.id)) walk(n.children)
    }
  }
  walk(roots)
  return out
}

/** id ของทุก node ที่มีลูก (ใช้ "ขยายทั้งหมด") */
export function branchIds(roots: readonly TreeNode[]): string[] {
  const out: string[] = []
  const walk = (list: readonly TreeNode[]) => {
    for (const n of list) {
      if (n.children.length) {
        out.push(n.account.id)
        walk(n.children)
      }
    }
  }
  walk(roots)
  return out
}

/** ค่าเริ่มต้น: ขยายเฉพาะระดับบนสุด (เห็นหมวด + กลุ่มบัญชีระดับแรก) */
export function initialExpanded(roots: readonly TreeNode[]): Set<string> {
  return new Set(roots.filter((n) => n.children.length).map((n) => n.account.id))
}
