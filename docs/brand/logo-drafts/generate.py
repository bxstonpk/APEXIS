"""APEXIS logo drafts v2 — แนว Gradient / Abstract mark ให้เข้าชุดกับภาพ login (glass)

สร้าง: <concept>-mark.svg (สัญลักษณ์ล้วน, ไม่มีตัวอักษร) + index.html (แผ่นนำเสนอ: lockup พื้นสว่าง / เข้ม, ไอคอนแอป, ทดสอบขนาดเล็ก)
รัน:  python generate.py   (เขียนไฟล์ลงโฟลเดอร์เดียวกับสคริปต์)
"""
import os

OUT = os.path.dirname(os.path.abspath(__file__))

NAVY, BLUE, SKY, INDIGO = '#101c3b', '#1d3f8f', '#4f86f7', '#2a4fc2'
ORANGE, AMBER, GOLD = '#f28c1a', '#ffb547', '#ffd08a'


def defs(p):
    """gradient / filter ชุดเดียวกันทุกแนว — prefix p กัน id ชนกันเมื่อวาง SVG หลายอันในหน้าเดียว"""
    return f'''<defs>
  <linearGradient id="{p}blue" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="{SKY}"/><stop offset="1" stop-color="{BLUE}"/></linearGradient>
  <linearGradient id="{p}deep" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="{INDIGO}"/><stop offset="1" stop-color="{NAVY}"/></linearGradient>
  <linearGradient id="{p}sky" x1="1" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#8fb4ff"/><stop offset=".55" stop-color="{SKY}"/><stop offset="1" stop-color="{INDIGO}"/></linearGradient>
  <linearGradient id="{p}mid" x1="1" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#3466dd"/><stop offset="1" stop-color="#1a3278"/></linearGradient>
  <linearGradient id="{p}fire" x1="0" y1="1" x2="1" y2="0"><stop offset="0" stop-color="#ff7a2f"/><stop offset=".55" stop-color="{ORANGE}"/><stop offset="1" stop-color="{GOLD}"/></linearGradient>
  <linearGradient id="{p}gloss" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#fff" stop-opacity=".55"/><stop offset=".5" stop-color="#fff" stop-opacity="0"/></linearGradient>
  <filter id="{p}glow" x="-50%" y="-50%" width="200%" height="200%"><feGaussianBlur stdDeviation="5"/></filter>
</defs>'''


# ---------- 4 แนวทาง (viewBox 0 0 120 120) ----------

def prism(p):
    """Glass Prism — ขา A สองชิ้นเป็นแผ่นกระจกไล่สีซ้อนกัน (ช่วงซ้อนเข้มขึ้นแบบกระจกจริง) + คานส้มเรืองแสง"""
    return defs(p) + f'''
<path d="M50 10 Q56 3 62 10 L102 106 Q104 114 96 114 H80 Q75 114 73 109 L41 32 Z" fill="url(#{p}sky)" opacity=".92"/>
<path d="M70 10 Q64 3 58 10 L18 106 Q16 114 24 114 H40 Q45 114 47 109 L79 32 Z" fill="url(#{p}deep)" opacity=".9" style="mix-blend-mode:multiply"/>
<path d="M70 10 Q64 3 58 10 L18 106 Q16 114 24 114 H40 Q45 114 47 109 L79 32 Z" fill="url(#{p}gloss)"/>
<rect x="26" y="74" width="70" height="11" rx="5.5" fill="url(#{p}fire)" filter="url(#{p}glow)" opacity=".75"/>
<rect x="26" y="74" width="70" height="11" rx="5.5" fill="url(#{p}fire)"/>
<rect x="31" y="75.5" width="44" height="3" rx="1.5" fill="#fff" opacity=".5"/>'''


def folded(p):
    """Folded Apex — ริบบิ้นพับเป็นตัว A: ด้านซ้ายมืด ด้านขวาสว่าง รอยพับมีเงา ยอดเป็นแผ่นส้ม (ดูมีมิติ)
    v2.1 (ที่เลือก): เงาเด่นขึ้น — เงาตกกระทบใต้ตัว A, drop shadow ทั้งชิ้น, ยอดส้มทอดเงาลงบนแผ่นน้ำเงิน, รอยพับเข้มขึ้น"""
    return defs(p) + f'''
<filter id="{p}drop" x="-30%" y="-30%" width="160%" height="170%"><feDropShadow dx="0" dy="4" stdDeviation="3.5" flood-color="{NAVY}" flood-opacity=".38"/></filter>
<filter id="{p}soft" x="-50%" y="-200%" width="200%" height="500%"><feGaussianBlur stdDeviation="3"/></filter>
<ellipse cx="60" cy="112" rx="46" ry="4.5" fill="{NAVY}" opacity=".3" filter="url(#{p}soft)"/>
<g filter="url(#{p}drop)">
<path d="M60 8 L14 110 H38 L60 58 Z" fill="url(#{p}mid)"/>
<path d="M60 8 L106 110 H82 L60 58 Z" fill="url(#{p}blue)"/>
<path d="M60 58 L82 110 H91 L63 52 Z" fill="{NAVY}" opacity=".45"/>
<path d="M44 44 L60 36 L76 44 L73.5 50 L60 43.5 L46.5 50 Z" fill="{NAVY}" opacity=".38"/>
<path d="M60 8 L106 110 H82 L60 58 Z" fill="url(#{p}gloss)" opacity=".7"/>
<path d="M60 8 L76 44 L60 36 L44 44 Z" fill="url(#{p}fire)"/>
<path d="M60 8 L76 44 L60 36 Z" fill="#fff" opacity=".2"/>
</g>'''


def ascend(p):
    """Ascend X — ตัว X ลายเซ็นของชื่อ: เส้นน้ำเงินกับเส้นส้มไขว้กัน ปลายเส้นส้มเป็นลูกศรพุ่งขึ้น (เติบโต)"""
    return defs(p) + f'''
<path d="M24 22 L96 98" stroke="url(#{p}blue)" stroke-width="22" stroke-linecap="round"/>
<path d="M24 22 L96 98" stroke="url(#{p}gloss)" stroke-width="22" stroke-linecap="round" opacity=".6"/>
<path d="M30 100 L82 30" stroke="url(#{p}fire)" stroke-width="20" stroke-linecap="round" filter="url(#{p}glow)" opacity=".6"/>
<path d="M30 100 L80 33" stroke="url(#{p}fire)" stroke-width="20" stroke-linecap="round"/>
<path d="M66 18 L100 14 L96 48 Z" fill="url(#{p}fire)" stroke="url(#{p}fire)" stroke-width="6" stroke-linejoin="round"/>
<circle cx="60" cy="60" r="7" fill="#fff" opacity=".9"/>'''


def orbit(p):
    """Orbit Apex — ต่อยอดโลโก้เดิม (A + วงโคจรส้ม) แบบเรียบ: A ไล่สีเต็มชิ้น วงโคจรวิ่งหน้า / หลังตัว A จริง"""
    return defs(p) + f'''
<clipPath id="{p}back"><rect x="0" y="0" width="120" height="62"/></clipPath>
<ellipse cx="60" cy="66" rx="54" ry="17" transform="rotate(-18 60 66)" fill="none" stroke="url(#{p}fire)" stroke-width="5" clip-path="url(#{p}back)" opacity=".55"/>
<path d="M60 10 Q64 10 66 14 L104 106 Q106 112 100 112 H84 Q80 112 78 108 L60 62 L42 108 Q40 112 36 112 H20 Q14 112 16 106 L54 14 Q56 10 60 10 Z" fill="url(#{p}blue)"/>
<path d="M60 10 Q64 10 66 14 L104 106 Q106 112 100 112 H84 Q80 112 78 108 L60 62 Z" fill="{NAVY}" opacity=".28"/>
<path d="M60 10 Q56 10 54 14 L16 106 Q14 112 20 112 H36 Q40 112 42 108 L60 62 Z" fill="url(#{p}gloss)" opacity=".8"/>
<clipPath id="{p}front"><rect x="0" y="62" width="120" height="58"/></clipPath>
<ellipse cx="60" cy="66" rx="54" ry="17" transform="rotate(-18 60 66)" fill="none" stroke="url(#{p}fire)" stroke-width="5" clip-path="url(#{p}front)"/>
<circle cx="108" cy="44" r="5" fill="{GOLD}"/><circle cx="108" cy="44" r="11" fill="{AMBER}" opacity=".3"/>'''


CONCEPTS = [
    ('1-glass-prism', 'Glass Prism', prism,
     'ขา A เป็นแผ่นกระจกไล่สีสองชิ้นซ้อนกัน คานส้มเรืองแสง — ภาษาเดียวกับภาพ login (glass) ดูพรีเมียม ทันสมัย'),
    ('2-folded-apex', 'Folded Apex', folded,
     'ริบบิ้นพับเป็นตัว A มีด้านมืด / สว่าง / รอยพับ ยอดเป็นแผ่นส้ม = Apex — มีมิติ จำง่าย ย่อเล็กแล้วยังเป็นตัว A'),
    ('3-ascend-x', 'Ascend X', ascend,
     'ใช้ตัว X ซึ่งเป็นลายเซ็นของชื่อ (APE·X·IS) เส้นส้มพุ่งเป็นลูกศรขึ้น = การเติบโต — โดดเด่นที่สุด ต่างจากโลโก้ ERP ทั่วไป'),
    ('4-orbit-apex', 'Orbit Apex', orbit,
     'ต่อยอดโลโก้ปัจจุบัน (A + วงโคจรส้ม) แต่เรียบและคมขึ้น วงโคจรผ่านหน้า / หลังตัว A จริง — เปลี่ยนแล้วผู้ใช้เดิมยังจำได้'),
]


def mark_svg(fn, p, size=None):
    wh = f' width="{size}" height="{size}"' if size else ''
    return f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 120 120"{wh} fill="none">{fn(p)}</svg>'


for key, _, fn, _ in CONCEPTS:
    open(os.path.join(OUT, f'{key}-mark.svg'), 'w', encoding='utf-8').write(mark_svg(fn, 'm') + '\n')

# ---------- แผ่นนำเสนอ ----------
cards = []
for i, (key, name, fn, desc) in enumerate(CONCEPTS):
    m = lambda tag, size=None: mark_svg(fn, f'c{i}{tag}', size)
    sizes = ''.join(f'<div class="sz">{m(f"s{s}", s)}<span>{s}px</span></div>' for s in (16, 24, 32, 48, 64))
    cards.append(f'''
<section class="card" id="{key}">
  <header><span class="num">{i + 1:02d}</span><div><h2>{name}</h2><p>{desc}</p></div></header>
  <div class="hero light">
    <div class="lockup">{m('l', 132)}<div class="word"><b>APE<i>X</i>IS</b><small>ENTERPRISE RESOURCE PLANNING</small></div></div>
  </div>
  <div class="row">
    <div class="hero dark small"><div class="lockup">{m('d', 92)}<div class="word"><b>APE<i>X</i>IS</b><small>ENTERPRISE RESOURCE PLANNING</small></div></div></div>
    <div class="tiles">
      <div class="app">{m('a', 68)}</div>
      <div class="app light">{m('b', 68)}</div>
      <div class="sizes">{sizes}</div>
    </div>
  </div>
</section>''')

html = f'''<!doctype html>
<html lang="th"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>APEXIS Logo Drafts</title>
<link rel="stylesheet" href="../banners/login-hero/fonts/fonts.css">
<style>
:root {{ --navy:{NAVY}; --line:#e4e7ec; --muted:#5a6376; --canvas:#eef1f6; }}
* {{ box-sizing:border-box; margin:0; padding:0; }}
body {{ background:var(--canvas); color:var(--navy); font-family:'Plex Thai', sans-serif; }}
main {{ max-width:1180px; margin:0 auto; padding:48px 24px 72px; display:flex; flex-direction:column; gap:28px; }}
.top h1 {{ font-size:30px; font-weight:700; letter-spacing:-.01em; }} .top p {{ margin-top:6px; color:var(--muted); font-size:15px; line-height:1.6; }}
.card {{ background:#fff; border:1px solid var(--line); border-radius:20px; padding:24px; display:flex; flex-direction:column; gap:16px; box-shadow:0 8px 30px rgba(16,28,59,.06); }}
.card header {{ display:flex; gap:16px; align-items:flex-start; }}
.num {{ font-size:13px; font-weight:700; color:#fff; background:var(--navy); border-radius:8px; padding:6px 9px; }}
.card h2 {{ font-size:20px; font-weight:700; }} .card header p {{ margin-top:4px; font-size:14.5px; color:var(--muted); line-height:1.6; }}
.hero {{ border-radius:16px; display:flex; align-items:center; justify-content:center; min-height:240px; border:1px solid var(--line);
  background:radial-gradient(60% 90% at 50% 0%, #f4f7ff, #fff); }}
.hero.dark {{ border-color:#0a1330; background:radial-gradient(70% 100% at 80% 0%, #24448f 0%, transparent 60%), radial-gradient(50% 80% at 100% 100%, rgba(242,140,26,.35), transparent 60%), #0a1330; }}
.hero.small {{ min-height:200px; }}
.lockup {{ display:flex; align-items:center; gap:22px; }}
.word b {{ display:block; font-weight:700; font-size:68px; line-height:1; letter-spacing:.04em; color:var(--navy); }}
.word b i {{ font-style:normal; background:linear-gradient(45deg, #ff7a2f, {ORANGE} 55%, {GOLD}); -webkit-background-clip:text; background-clip:text; color:transparent; }}
.word small {{ display:block; margin-top:10px; font-size:12.5px; font-weight:500; letter-spacing:.34em; color:var(--muted); }}
.small .word b {{ font-size:48px; }} .small .word small {{ font-size:10px; margin-top:8px; }}
.dark .word b {{ color:#fff; }} .dark .word small {{ color:rgba(255,255,255,.6); }}
.row {{ display:grid; grid-template-columns:1.25fr 1fr; gap:16px; }}
.tiles {{ display:grid; grid-template-columns:auto auto 1fr; gap:16px; align-items:center; border:1px solid var(--line); border-radius:16px; padding:16px; }}
.app {{ width:104px; height:104px; border-radius:26px; display:flex; align-items:center; justify-content:center;
  background:radial-gradient(80% 80% at 30% 0%, #2a4fc2, #101c3b 70%); box-shadow:0 10px 24px rgba(16,28,59,.3), inset 0 1px 0 rgba(255,255,255,.2); }}
.app.light {{ background:linear-gradient(180deg, #fff, #f1f4fa); box-shadow:0 10px 24px rgba(16,28,59,.12), inset 0 0 0 1px var(--line); }}
.sizes {{ display:flex; align-items:flex-end; gap:14px; justify-content:center; flex-wrap:wrap; }}
.sz {{ display:flex; flex-direction:column; align-items:center; gap:6px; }} .sz span {{ font-size:11px; color:var(--muted); }}
@media (max-width:860px) {{ .row {{ grid-template-columns:1fr; }} .lockup {{ flex-direction:column; text-align:center; }} .word b {{ font-size:48px; }} }}
</style></head>
<body><main>
<div class="top"><h1>APEXIS — โลโก้ draft v2</h1>
<p>แนว Gradient / Abstract mark ให้เข้าชุดกับภาพหน้า login (glass) · สีกรมท่า {NAVY} · น้ำเงิน {BLUE} → {SKY} · ส้ม {ORANGE} → {GOLD}<br>
แต่ละแนว: แนวนอนพื้นสว่าง / พื้นเข้ม · ไอคอนแอป · ทดสอบขนาด 16–64 px · ตัวอักษร APEXIS ใช้ IBM Plex Sans Thai (ฟอนต์เดียวกับระบบ)</p></div>
{''.join(cards)}
</main></body></html>
'''
open(os.path.join(OUT, 'index.html'), 'w', encoding='utf-8').write(html)
print('ok')
