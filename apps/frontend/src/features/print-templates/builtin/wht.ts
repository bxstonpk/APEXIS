/**
 * แม่แบบมาตรฐาน หนังสือรับรองการหักภาษี ณ ที่จ่าย (50 ทวิ) — พิมพ์ทุกฉบับใน copies (ฉบับที่ 1 / 2) แผ่นละ 1 หน้า
 * ข้อมูลรูปแบบ WhtCertificateData (finance/WhtCertificate.tsx: whtCertificateData)
 */
export const WHT_CERTIFICATE_TEMPLATE = `<style>
  .wht { font-size: 12px; }
  .wht p, .wht h1 { margin: 0; }
  .wht .top { display: flex; justify-content: space-between; gap: 16px; align-items: flex-start; }
  .wht .copy { max-width: 60%; font-size: 11px; color: #525252; }
  .wht .bookno { display: grid; grid-template-columns: auto auto; column-gap: 8px; margin: 0; }
  .wht .bookno dt { color: #525252; } .wht .bookno dd { margin: 0; font-variant-numeric: tabular-nums; }
  .wht .center { text-align: center; margin-top: 8px; }
  .wht h1 { font-size: 18px; font-weight: 700; }
  .wht .box { margin-top: 12px; border: 1px solid #737373; border-radius: 4px; padding: 8px 12px; }
  .wht .row { display: flex; flex-wrap: wrap; justify-content: space-between; align-items: baseline; column-gap: 16px; }
  .wht .muted { color: #525252; }
  .wht .strong { font-weight: 600; }
  .wht .taxid { font-weight: 600; letter-spacing: .06em; font-variant-numeric: tabular-nums; }
  .wht .checks { display: flex; flex-wrap: wrap; align-items: center; column-gap: 20px; row-gap: 4px; }
  .wht .check { display: inline-flex; align-items: center; gap: 6px; }
  .wht .tick { display: inline-flex; width: 14px; height: 14px; align-items: center; justify-content: center; border: 1px solid #404040; font-size: 10px; font-weight: 700; line-height: 1; }
  .wht .seq { display: inline-block; width: 4rem; border-bottom: 1px dotted #737373; text-align: center; font-weight: 600; }
  .wht table { width: 100%; margin-top: 12px; border: 1px solid #737373; }
  .wht th { background: #f5f5f5; border: 1px solid #737373; padding: 6px 8px; font-size: 11.5px; font-weight: 600; }
  .wht td { border-left: 1px solid #737373; border-right: 1px solid #737373; padding: 6px 8px; vertical-align: top; }
  .wht .num { text-align: right; font-variant-numeric: tabular-nums; }
  .wht .names { display: block; font-size: 11px; color: #525252; }
  .wht .sum td { border: 1px solid #737373; font-weight: 600; }
  .wht .dbl { border-bottom: 4px double #737373 !important; }
  .wht .words { margin-left: 8px; background: #f5f5f5; border-radius: 4px; padding: 2px 8px; font-weight: 600; }
  .wht .small { margin-top: 8px; font-size: 11px; color: #404040; }
  .wht .foot { margin-top: auto; padding-top: 32px; display: grid; grid-template-columns: 1fr auto; gap: 24px; align-items: end; break-inside: avoid; }
  .wht .signer { width: 18rem; text-align: center; }
  .wht .signline { margin-top: 40px; border-bottom: 1px dotted #525252; }
  .wht .seal { margin: 8px auto 0; width: 64px; height: 64px; border: 1px dashed #a3a3a3; border-radius: 50%; display: flex; align-items: center; justify-content: center; font-size: 10px; color: #737373; }
</style>
{{#each copies}}
<div class="sheet wht">
  <div class="top">
    <p class="copy">{{label}}</p>
    <dl class="bookno"><dt>{{../labels.book}}</dt><dd>{{../book}}</dd><dt>{{../labels.number}}</dt><dd class="strong">{{../number}}</dd></dl>
  </div>
  <div class="center"><h1>{{../title}}</h1><p>{{../subtitle}}</p></div>

  {{#with ../payer}}
  <div class="box">
    <div class="row"><p class="strong">{{label}}</p><p>{{../../labels.tax_id}} <span class="taxid">{{#if tax_id}}{{tax_id}}{{else}}-{{/if}}</span> <span class="muted">{{branch_label}}</span></p></div>
    <p style="margin-top:4px"><span class="muted">{{../../labels.name}}</span> {{name}}</p>
    <p><span class="muted">{{../../labels.address}}</span> {{#if address}}{{address}}{{else}}-{{/if}}</p>
  </div>
  {{/with}}
  {{#with ../payee}}
  <div class="box">
    <div class="row"><p class="strong">{{label}}</p><p>{{../../labels.tax_id}} <span class="taxid">{{#if tax_id}}{{tax_id}}{{else}}-{{/if}}</span> <span class="muted">{{branch_label}}</span></p></div>
    <p style="margin-top:4px"><span class="muted">{{../../labels.name}}</span> {{name}}</p>
    <p><span class="muted">{{../../labels.address}}</span> {{#if address}}{{address}}{{else}}-{{/if}}</p>
  </div>
  {{/with}}

  <div class="box checks">
    <span>{{../labels.sequence}} <span class="seq">{{../sequence}}</span> {{../labels.in_form}}</span>
    {{#each ../forms}}<span class="check"><span class="tick">{{#if checked}}✓{{/if}}</span><span class="{{#if checked}}strong{{/if}}">{{label}}</span></span>{{/each}}
  </div>

  <table>
    <thead><tr><th style="text-align:left">{{../labels.col_income}}</th><th style="width:7rem">{{../labels.col_date}}</th><th style="width:8rem">{{../labels.col_amount}}</th><th style="width:8rem">{{../labels.col_tax}}</th></tr></thead>
    <tbody>
      {{#each ../incomes}}
      <tr><td>{{label}}{{#if names}}<span class="names">({{names}})</span>{{/if}}</td><td style="text-align:center">{{#if has}}{{date ../../date}}{{/if}}</td><td class="num">{{#if has}}{{money amount}}{{/if}}</td><td class="num">{{#if has}}{{money tax}}{{/if}}</td></tr>
      {{/each}}
      <tr class="sum"><td colspan="2" class="num">{{../labels.total}}</td><td class="num">{{money ../total_amount}}</td><td class="num dbl">{{money ../total_tax}}</td></tr>
      <tr class="sum"><td colspan="4" style="font-weight:400">{{../labels.total_in_words}} <span class="words">({{bahtText ../total_tax}})</span></td></tr>
    </tbody>
  </table>

  <p class="small">{{../labels.funds}}</p>

  <div class="box checks">
    <span class="strong">{{../labels.payer_mode}}</span>
    {{#each ../modes}}<span class="check"><span class="tick">{{#if checked}}✓{{/if}}</span><span class="{{#if checked}}strong{{/if}}">{{label}}</span></span>{{/each}}
  </div>

  <div class="foot">
    <p class="small">{{../labels.warning}}</p>
    <div class="signer">
      <p>{{../labels.certify}}</p>
      <p class="signline"></p>
      <p style="margin-top:4px">{{../labels.signer}}</p>
      <p style="margin-top:4px">{{../issued_on}}</p>
      <div class="seal">{{../labels.seal}}</div>
    </div>
  </div>
</div>
{{/each}}`
