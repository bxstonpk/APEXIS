import { fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { texts } from '@/config'
import { checkAsset } from './AssetManager'
import { builtinTemplate, familyOf } from './builtin'
import { sampleData, VARIABLES } from './catalog'
import { documentPrintData } from './data'
import { renderDocument, renderTemplate, sanitize } from './engine'
import { readHtmlFile, TemplateEditor } from './TemplateEditor'
import { TEMPLATE_DOC_TYPES } from './types'

vi.mock('next/navigation', () => ({ useRouter: () => ({ push: vi.fn(), replace: vi.fn(), refresh: vi.fn() }) }))

afterEach(() => {
  vi.unstubAllGlobals()
  document.body.innerHTML = ''
})

const t = texts.printTemplates

describe('engine', () => {
  it('helpers: money / qty / date / bahtText / inc / eq', () => {
    const html = renderTemplate('{{money a}}|{{qty q}}|{{date d}}|{{bahtText a}}|{{#each xs}}{{inc @index}}{{/each}}|{{#if (eq k "x")}}yes{{/if}}', {
      a: '1234.5',
      q: '2.500',
      d: '2026-11-26',
      xs: [1, 2],
      k: 'x',
    })
    const [money, qty, date, words, idx, eq] = html.split('|')
    expect(money).toBe('1,234.50')
    expect(qty).toBe('2.5')
    expect(date).toContain('2569')
    expect(words).toContain('หนึ่งพันสองร้อยสามสิบสี่บาท')
    expect(idx).toBe('12')
    expect(eq).toBe('yes')
  })

  it('escape ค่าเสมอ — ค่าที่มี <script> เป็นข้อความ', () => {
    const out = renderTemplate('<p>{{name}}</p>', { name: '<script>alert(1)</script>' })
    expect(out).toBe('<p>&lt;script&gt;alert(1)&lt;/script&gt;</p>')
  })

  it('ไม่ให้เข้าถึง prototype', () => {
    expect(renderTemplate('{{constructor.name}}', {})).toBe('')
  })

  it('sanitize ตัด script / event handler / javascript: ออก แต่คง style', () => {
    const out = sanitize('<style>.a{color:red}</style><p class="a" onclick="x()">ok</p><script>alert(1)</script><a href="javascript:alert(1)">l</a>')
    expect(out).not.toContain('<script')
    expect(out).not.toContain('onclick')
    expect(out).not.toContain('javascript:')
    expect(out).toContain('<style>')
    expect(out).toContain('ok')
  })

  it('renderDocument ใส่กระดาษตาม paper และ asset ตามชื่อ', () => {
    const doc = renderDocument('<div class="sheet"><img src="{{asset "logo"}}" alt=""></div>', 'a4_landscape', {}, { logo: '/api/v1/companies/c/document-assets/1/content' })
    expect(doc).toContain('size:A4 landscape')
    expect(doc).toContain('src="/api/v1/companies/c/document-assets/1/content"')
  })
})

describe('ข้อมูลพิมพ์ + แม่แบบมาตรฐาน', () => {
  const letterhead = {
    name: 'Demo',
    legal_name_th: 'บริษัท เดโม จำกัด',
    legal_name_en: '',
    tax_id: '0105555000001',
    address: { line: '1 ถนนสีลม', sub_district: 'สีลม', district: 'บางรัก', province: 'กรุงเทพมหานคร', postal_code: '10500', country: 'TH' },
    phone: '02-1',
    email: '',
    website: '',
  }

  it('documentPrintData: คอลัมน์ราคา, ยอดเป็นตัวอักษร, สาขาคู่ค้า', () => {
    const d = documentPrintData({
      letterhead: letterhead as never,
      title: 'ใบกำกับภาษี',
      docNo: 'IV-1',
      info: [['วันที่', '26 พ.ย. 2569'], ['อ้างอิง', '']],
      party: { label: 'ลูกค้า', name: 'ลูกค้า ก', taxId: '0105561000011', branchNo: '00001' },
      lines: [{ description: 'สินค้า', quantity: '2', unitPrice: '500', amount: '1000' }],
      totals: [{ label: 'รวม', value: '1070', strong: true }],
      amountInWords: '1070',
      signatures: ['ผู้รับ'],
    })
    expect(d.columns.map((c) => c.label)).toEqual([texts.sales.print.seq, texts.sales.print.itemDesc, texts.sales.print.qty, texts.sales.print.unitPrice, texts.sales.print.amount])
    expect(d.info).toEqual([{ label: 'วันที่', value: '26 พ.ย. 2569' }])
    expect(d.party?.branch_label).toContain('00001')
    expect(d.amount_in_words).toBe('หนึ่งพันเจ็ดสิบบาทถ้วน')
    expect(d.lines[0]!.cells.map((c) => c.value)).toEqual(['1', 'สินค้า', '2.00', '500.00', '1,000.00'])
  })

  it('แม่แบบมาตรฐานเอกสารทั่วไปแสดงหัวกระดาษ / เลขที่ / รายการ / ยอดตัวอักษร', () => {
    const el = document.createElement('div')
    el.innerHTML = sanitize(renderTemplate(builtinTemplate('sales.invoice').html, sampleData('sales.invoice')))
    document.body.appendChild(el)
    expect(screen.getByText('IV2026-000001')).toBeInTheDocument()
    expect(screen.getByText('สินค้า A')).toBeInTheDocument()
    expect(screen.getByText('1,391.00')).toBeInTheDocument()
    expect(screen.getByText('(หนึ่งพันสามร้อยเก้าสิบเอ็ดบาทถ้วน)')).toBeInTheDocument()
  })

  it('ทุกชนิดเอกสาร: แม่แบบมาตรฐาน render กับข้อมูลตัวอย่างได้ และมีรายการตัวแปร', () => {
    for (const dt of TEMPLATE_DOC_TYPES) {
      const { html, paper } = builtinTemplate(dt)
      expect(() => renderDocument(html, paper, sampleData(dt))).not.toThrow()
      expect(VARIABLES[familyOf(dt)].length).toBeGreaterThan(5)
    }
    expect(builtinTemplate('finance.pnd53').paper).toBe('a4_landscape')
    expect(builtinTemplate('finance.wht_certificate').html).toContain('{{#each copies}}')
  })
})

describe('หน้าแก้แม่แบบ', () => {
  const initial = { docType: 'sales.invoice' as const, name: '', html: '<p>{{title}}</p>', paper: 'a4_portrait' as const, status: 'active' as const }

  it('นำเข้าไฟล์: ต้องเป็น .html ไม่เกิน 200 KB', async () => {
    expect(await readHtmlFile(new File(['x'], 'a.txt', { type: 'text/plain' }))).toEqual({ error: t.errors.fileType })
    expect(await readHtmlFile(new File(['a'.repeat(201 * 1024)], 'a.html', { type: 'text/html' }))).toEqual({ error: t.errors.fileTooLarge })
    expect(await readHtmlFile(new File(['<p>ok</p>'], 'a.html', { type: 'text/html' }))).toEqual({ html: '<p>ok</p>' })
  })

  it('คลิกตัวแปรแทรกที่ตำแหน่งเคอร์เซอร์ และบันทึกส่ง doc_type / html', async () => {
    const fetch = vi.fn(async (_url: string, init?: RequestInit) =>
      init?.method === 'POST'
        ? new Response(JSON.stringify({ data: { id: 't1' } }), { status: 201, headers: { 'Content-Type': 'application/json' } })
        : new Response(JSON.stringify({ data: [] }), { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )
    vi.stubGlobal('fetch', fetch)
    render(<TemplateEditor companyId="c1" listHref="/c/c1/organization/templates" canManage initial={initial} />)
    const code = screen.getByLabelText(t.fields.html) as HTMLTextAreaElement
    code.focus()
    code.setSelectionRange(3, 3)
    fireEvent.click(screen.getByRole('button', { name: `${t.actions.insert} document_no` }))
    expect(code.value).toBe('<p>{{document_no}}{{title}}</p>')
    fireEvent.change(screen.getByLabelText(new RegExp(t.fields.name)), { target: { value: 'แบบบริษัท' } })
    fireEvent.click(screen.getByRole('button', { name: t.actions.save }))
    await vi.waitFor(() => expect(fetch.mock.calls.some(([, i]) => (i as RequestInit | undefined)?.method === 'POST')).toBe(true))
    const [url, init] = fetch.mock.calls.find(([, i]) => (i as RequestInit | undefined)?.method === 'POST')! as [string, RequestInit]
    expect(url).toContain('/companies/c1/document-templates')
    expect(JSON.parse(String(init.body))).toMatchObject({ doc_type: 'sales.invoice', name: 'แบบบริษัท', html: '<p>{{document_no}}{{title}}</p>', paper: 'a4_portrait' })
  })

  it('ไม่มีชื่อ = ไม่ส่ง และแจ้งที่ช่อง', () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({ data: [] }), { status: 200, headers: { 'Content-Type': 'application/json' } })))
    render(<TemplateEditor companyId="c1" listHref="/x" canManage initial={initial} />)
    fireEvent.click(screen.getByRole('button', { name: t.actions.save }))
    expect(screen.getByText(t.errors.nameRequired)).toBeInTheDocument()
  })

  it('รูป: ชื่อและชนิด / ขนาดไฟล์', () => {
    expect(checkAsset('Logo', new File(['x'], 'a.png', { type: 'image/png' }))).toHaveProperty('name')
    expect(checkAsset('logo', new File(['x'], 'a.gif', { type: 'image/gif' }))).toHaveProperty('file', t.errors.assetType)
    expect(checkAsset('logo', new File(['x'], 'a.png', { type: 'image/png' }))).toEqual({})
  })
})
