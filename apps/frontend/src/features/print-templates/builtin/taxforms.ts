/** แม่แบบมาตรฐานของแบบภาษีรายเดือน (ภ.ง.ด.3 / 53, ภ.พ.30, รายงานภาษีขาย / ซื้อ) — ข้อมูลจาก finance/TaxForms.tsx */

const BASE_CSS = `
  .tf { font-size: 11px; }
  .tf p, .tf h1 { margin: 0; }
  .tf .head { display: flex; justify-content: space-between; align-items: flex-start; gap: 24px; }
  .tf .payer { display: grid; grid-template-columns: auto 1fr; column-gap: 12px; row-gap: 2px; font-size: 12px; margin: 0; }
  .tf .payer dt { color: #525252; } .tf .payer dd { margin: 0; }
  .tf .right { text-align: right; }
  .tf h1 { font-size: 16px; font-weight: 700; }
  .tf .muted { color: #525252; }
  .tf table { width: 100%; margin-top: 12px; }
  .tf th { background: #f5f5f5; border: 1px solid #737373; padding: 4px 6px; font-size: 10.5px; }
  .tf td { border: 1px solid #737373; padding: 4px 6px; vertical-align: top; break-inside: avoid; }
  .tf .num { text-align: right; white-space: nowrap; font-variant-numeric: tabular-nums; }
  .tf .c { text-align: center; }
  .tf .sub { display: block; font-size: 10px; color: #525252; }
  .tf .b { font-weight: 600; } .tf .bb { font-weight: 700; }
  .tf .empty { padding: 24px 6px; text-align: center; color: #737373; }`

const PAYER = `<dl class="payer">
      <dt>{{payer.label}}</dt><dd class="b">{{payer.name}}</dd>
      <dt>{{labels.tax_id}}</dt><dd>{{#if payer.tax_id}}{{payer.tax_id}}{{else}}-{{/if}} · {{labels.branch}} {{payer.branch_label}}</dd>
      <dt>{{labels.address}}</dt><dd>{{#if payer.address}}{{payer.address}}{{else}}-{{/if}}</dd>
    </dl>`

/** ใบแนบ ภ.ง.ด.3 / 53 — A4 แนวนอน แผ่นละ 12 บรรทัด (pages คำนวณไว้แล้ว) */
export const WITHHOLDING_TEMPLATE = `<style>${BASE_CSS}</style>
{{#each pages}}
<div class="sheet tf">
  <div class="head">
    {{#with ..}}${PAYER}{{/with}}
    <div class="right"><h1>{{../title}}</h1><p style="font-size:12px">{{../period}}</p><p class="muted" style="margin-top:4px">{{page_label}}</p></div>
  </div>
  <table>
    <thead><tr>
      <th style="width:2.5rem">{{../labels.seq}}</th><th style="width:8rem">{{../labels.tax_id}}</th><th>{{../labels.name_address}}</th>
      <th style="width:5rem">{{../labels.date}}</th><th style="width:9rem">{{../labels.income}}</th><th style="width:3rem">{{../labels.rate}}</th>
      <th style="width:6rem">{{../labels.base}}</th><th style="width:5rem">{{../labels.wht}}</th><th style="width:3rem">{{../labels.condition}}</th>
    </tr></thead>
    <tbody>
      {{#unless rows.length}}<tr><td colspan="9" class="empty">{{../labels.no_rows}}</td></tr>{{/unless}}
      {{#each rows}}
      <tr>
        <td class="c">{{#if first}}{{seq}}{{/if}}</td>
        <td>{{#if first}}{{tax_id}}<span class="sub">{{branch_label}}</span>{{/if}}</td>
        <td>{{#if first}}<span class="b">{{name}}</span><span class="sub">{{address}}</span>{{/if}}</td>
        <td class="c">{{#if first}}{{date date}}{{/if}}</td>
        <td>{{income}}</td><td class="num">{{rate}}</td><td class="num">{{money base}}</td><td class="num">{{money wht}}</td><td class="c">{{condition}}</td>
      </tr>
      {{/each}}
    </tbody>
    <tfoot>
      <tr class="b"><td colspan="6" class="num">{{../labels.page_total}}</td><td class="num">{{money page_base}}</td><td class="num">{{money page_wht}}</td><td></td></tr>
      {{#if last}}<tr class="bb"><td colspan="6" class="num">{{../labels.grand_total}} ({{../summary.payees_label}})</td><td class="num">{{money ../summary.base}}</td><td class="num">{{money ../summary.wht}}</td><td></td></tr>{{/if}}
    </tfoot>
  </table>
  {{#if last}}<p class="right" style="margin-top:8px">({{bahtText ../summary.wht}})</p>{{/if}}
</div>
{{/each}}`

/** ภ.พ.30 — A4 แนวตั้ง */
export const PP30_TEMPLATE = `<style>${BASE_CSS}
  .pp30 { font-size: 12px; gap: 16px; }
  .pp30 .title { text-align: center; } .pp30 .title h1 { font-size: 17px; }
  .pp30 table { margin-top: 0; border: 1px solid #737373; }
  .pp30 td, .pp30 th { border: 0; border-bottom: 1px solid #a3a3a3; background: none; padding: 10px 12px; font-size: 13px; text-align: left; font-weight: 400; }
  .pp30 tr:last-child td, .pp30 tr:last-child th { border-bottom: 0; }
  .pp30 .no { width: 2.5rem; text-align: right; color: #525252; }
  .pp30 .strong { font-weight: 600; }
</style>
<div class="sheet tf pp30">
  <div class="title"><h1>{{title}}</h1><p>{{period}}</p></div>
  ${PAYER}
  <table><tbody>
    {{#each rows}}<tr><td class="no">{{no}}.</td><th class="{{#if strong}}strong{{/if}}">{{label}}</th><td class="num {{#if strong}}strong{{/if}}">{{money value}}</td></tr>{{/each}}
  </tbody></table>
  <p class="right b">{{net_text}}</p>
</div>`

/** รายงานภาษีขาย / ซื้อ — A4 แนวนอน แผ่นละ 20 บรรทัด */
export const VAT_REPORT_TEMPLATE = `<style>${BASE_CSS}</style>
{{#each pages}}
<div class="sheet tf">
  <div class="head">
    {{#with ..}}${PAYER}{{/with}}
    <div class="right"><h1>{{../title}}</h1><p style="font-size:12px">{{../period}}</p><p class="muted" style="margin-top:4px">{{page_label}}</p></div>
  </div>
  <table>
    <thead><tr>
      <th style="width:2.5rem">{{../labels.seq}}</th><th style="width:5rem">{{../labels.date}}</th><th style="width:8rem">{{../labels.no}}</th><th>{{../labels.party}}</th>
      <th style="width:8rem">{{../labels.tax_id}}</th><th style="width:6rem">{{../labels.branch}}</th><th style="width:7rem">{{../labels.base}}</th><th style="width:6rem">{{../labels.tax}}</th>
    </tr></thead>
    <tbody>
      {{#unless rows.length}}<tr><td colspan="8" class="empty">{{../labels.no_rows}}</td></tr>{{/unless}}
      {{#each rows}}<tr><td class="c">{{seq}}</td><td class="c">{{date date}}</td><td>{{document_no}}</td><td>{{party_name}}</td><td>{{tax_id}}</td><td>{{branch_label}}</td><td class="num">{{money base}}</td><td class="num">{{money tax}}</td></tr>{{/each}}
    </tbody>
    <tfoot>
      <tr class="b"><td colspan="6" class="num">{{../labels.page_total}}</td><td class="num">{{money page_base}}</td><td class="num">{{money page_tax}}</td></tr>
      {{#if last}}<tr class="bb"><td colspan="6" class="num">{{../labels.grand_total}}</td><td class="num">{{money ../base}}</td><td class="num">{{money ../tax}}</td></tr>{{/if}}
    </tfoot>
  </table>
</div>
{{/each}}`
