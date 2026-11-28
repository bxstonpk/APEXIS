/**
 * แม่แบบมาตรฐาน "เอกสารทั่วไป" — หน้าตาเดียวกับหน้าพิมพ์เดิม (PrintDocument):
 * หัวกระดาษบริษัท + ชื่อเอกสาร / ต้นฉบับ, กล่องคู่ค้า + ข้อมูลเอกสาร, ตารางรายการ, ยอดเป็นตัวอักษร + ยอดรวม, ช่องลงนาม
 * ใช้ข้อมูลรูปแบบ DocumentPrintData (คอลัมน์ / ช่องในตารางมาจากข้อมูล จึงใช้ได้กับทุกเอกสารชนิดนี้)
 */
export const DOCUMENT_TEMPLATE = `<style>
  .doc h1, .doc p { margin: 0; }
  .doc .head { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; padding-bottom: 16px; border-bottom: 2px solid #262626; }
  .doc .head .company { min-width: 0; }
  .doc .legal { font-size: 17px; font-weight: 700; }
  .doc .muted { color: #525252; }
  .doc .title { flex-shrink: 0; text-align: right; }
  .doc .title h1 { font-size: 20px; font-weight: 700; letter-spacing: -0.01em; }
  .doc .copy { display: inline-block; margin-top: 2px; padding: 0 8px; border: 1px solid #737373; border-radius: 4px; font-size: 12px; }
  .doc .meta { display: grid; grid-template-columns: 1fr auto; gap: 16px; margin-top: 16px; }
  .doc .box { border: 1px solid #a3a3a3; border-radius: 4px; padding: 12px; }
  .doc .box .label { font-size: 12px; color: #525252; }
  .doc .party-name { font-weight: 600; }
  .doc .pre { white-space: pre-line; }
  .doc .facts { display: grid; grid-template-columns: auto 1fr; align-content: start; column-gap: 16px; row-gap: 4px; min-width: 14rem; }
  .doc .facts dt { color: #525252; }
  .doc .facts dd { margin: 0; text-align: right; font-weight: 500; font-variant-numeric: tabular-nums; }
  .doc table.lines { width: 100%; margin-top: 16px; }
  .doc table.lines th { background: #f5f5f5; border-bottom: 1px solid #a3a3a3; padding: 6px 8px; font-weight: 600; text-align: left; }
  .doc table.lines td { padding: 6px 8px; border-bottom: 1px solid #e5e5e5; vertical-align: top; break-inside: avoid; }
  .doc .center { text-align: center; }
  .doc .right { text-align: right; font-variant-numeric: tabular-nums; }
  .doc .detail { display: block; font-size: 12px; color: #525252; }
  .doc .blank { display: block; height: 16px; border-bottom: 1px dotted #737373; }
  .doc .sum { display: grid; grid-template-columns: 1fr 18rem; gap: 12px; margin-top: 12px; break-inside: avoid; }
  .doc .words { background: #f5f5f5; border-radius: 4px; padding: 8px 12px; text-align: center; font-weight: 600; }
  .doc .note { margin-top: 8px; font-size: 12px; color: #404040; }
  .doc .totals { display: grid; grid-template-columns: 1fr auto; margin: 0; }
  .doc .totals dt, .doc .totals dd { margin: 0; padding: 4px 0; }
  .doc .totals dd { padding-left: 16px; text-align: right; font-variant-numeric: tabular-nums; }
  .doc .totals .strong { font-weight: 700; border-top: 1px solid #404040; border-bottom: 4px double #404040; }
  .doc .signs { margin-top: auto; padding-top: 56px; display: grid; grid-template-columns: repeat(auto-fit, minmax(0, 1fr)); gap: 32px; break-inside: avoid; }
  .doc .sign { text-align: center; }
  .doc .sign .line { margin: 0 auto; height: 40px; width: 14rem; max-width: 100%; border-bottom: 1px dotted #525252; }
  .doc .sign .date { font-size: 12px; color: #525252; }
</style>
<div class="sheet doc">
  <div class="head">
    <div class="company">
      <p class="legal">{{company.legal_name_th}}</p>
      {{#if company.legal_name_en}}<p class="muted">{{company.legal_name_en}}</p>{{/if}}
      <p style="margin-top:4px">{{company.address}}</p>
      {{#if company.tax_id}}<p>{{labels.tax_id}} {{company.tax_id}} ({{company.branch_label}})</p>{{/if}}
      {{#if company.contact}}<p class="muted">{{company.contact}}</p>{{/if}}
    </div>
    <div class="title">
      <h1>{{title}}</h1>
      {{#if copy_label}}<span class="copy">{{copy_label}}</span>{{/if}}
    </div>
  </div>

  <div class="meta">
    {{#if party}}
    <div class="box">
      <p class="label">{{party.label}}</p>
      <p class="party-name">{{party.name}}</p>
      {{#if party.address}}<p class="pre">{{party.address}}</p>{{/if}}
      {{#if party.tax_id}}<p>{{labels.tax_id}} {{party.tax_id}}{{#if party.branch_label}} ({{party.branch_label}}){{/if}}</p>{{/if}}
      {{#each party.extra}}<p>{{this}}</p>{{/each}}
    </div>
    {{else}}<div></div>{{/if}}
    <dl class="box facts">
      <dt>{{labels.no}}</dt><dd>{{document_no}}</dd>
      {{#each info}}<dt>{{label}}</dt><dd>{{value}}</dd>{{/each}}
    </dl>
  </div>

  <table class="lines">
    <thead><tr>{{#each columns}}<th class="{{align}}"{{#if width}} style="width:{{width}}"{{/if}}>{{label}}</th>{{/each}}</tr></thead>
    <tbody>
      {{#each lines}}
      <tr>{{#each cells}}<td class="{{align}}">{{#if blank}}<span class="blank"></span>{{else}}{{value}}{{#if detail}}<span class="detail">{{detail}}</span>{{/if}}{{/if}}</td>{{/each}}</tr>
      {{/each}}
    </tbody>
  </table>

  {{#if totals.length}}
  <div class="sum">
    <div>
      {{#if amount_in_words}}<p class="words">({{amount_in_words}})</p>{{/if}}
      {{#if note}}<p class="note">{{note}}</p>{{/if}}
    </div>
    <dl class="totals">
      {{#each totals}}<dt class="{{#if strong}}strong{{/if}}">{{label}}</dt><dd class="{{#if strong}}strong{{/if}}">{{value}}</dd>{{/each}}
    </dl>
  </div>
  {{else}}
  {{#if note}}<p class="note">{{note}}</p>{{/if}}
  {{/if}}

  <div class="signs">
    {{#each signatures}}
    <div class="sign"><div class="line"></div><p style="margin-top:4px">{{label}}</p><p class="date">{{../labels.sign_date}}</p></div>
    {{/each}}
  </div>
</div>`
