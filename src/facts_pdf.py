# -*- coding: utf-8 -*-
"""«Сервер в фактах»: весёлая инфографика по данным spark-профиля. HTML под печать в PDF."""
import json, os, html, math, datetime

HERE = os.path.dirname(os.path.abspath(__file__))
F = json.load(open(os.path.join(HERE, 'facts.json'), encoding='utf-8'))
E = html.escape
ENT = F['entities']
W = F['windows'][0]
K = lambda d, k: d.get(str(k), d.get(k))
F.setdefault('generated_iso', datetime.datetime.fromtimestamp(
    F['generated_ms'] / 1000, datetime.timezone(datetime.timedelta(hours=3))
).strftime('%d.%m.%Y %H:%M МСК'))

CHUNKS = K(W, 10)
TILES = K(W, 9)
TOTAL_ENT = F['total_entities']
MONSTERS = sum(ENT.get(k, 0) for k in
               ('zombie', 'skeleton', 'creeper', 'enderman', 'spider', 'drowned', 'pillager', 'zombie_nautilus'))
FARM = sum(ENT.get(k, 0) for k in ('chicken', 'sheep', 'cow', 'pig'))
PETS = sum(ENT.get(k, 0) for k in ('cat', 'wolf', 'parrot', 'horse'))
AREA_M2 = CHUNKS * 256
FIELDS = AREA_M2 / 7140
BLOCKS = CHUNKS * 16 * 16 * 384
GC = F['gc'][0]
GC_PER_DAY = 86400 / (GC['avg_freq'] / 1000)
GC_DAY_SEC = GC_PER_DAY * GC['avg_time'] / 1000
TX_MONTH_GIB = F['net']['eth0']['2']['mean'] * 86400 * 30 / 1024 ** 3
MOVIES = TX_MONTH_GIB * 1024 ** 3 / (4 * 1000 ** 3)
CPU_SHARE = F['cpu']['threads'] / 48 * 100
RAM_SHARE = 2070 * 1024 ** 2 / F['mem']['phys_total'] * 100
DIST = math.hypot(904.2086179107224, 1677.5535255945756)
WALK_MIN = DIST / 4.317 / 60
TICKS_LIVED = F['uptime_ms'] / 1000 * 20


def n(x, d=0):
    return f"{x:,.{d}f}".replace(',', ' ').replace('.', ',')


CSS = """
@page { size: A4; margin: 0; }
* { box-sizing: border-box; }
html, body { margin: 0; background: #E7E3D9; }
body { font-family: "Inter", sans-serif; color: #1C212B; -webkit-font-smoothing: antialiased; }
.page { position: relative; width: 794px; height: 1123px; padding: 38px 44px 34px; background: #F7F4EC;
  overflow: hidden; page-break-after: always; break-after: page; }
.page:last-child { page-break-after: auto; break-after: auto; }
.page::before { content: ""; position: absolute; inset: 0; pointer-events: none;
  background-image: linear-gradient(rgba(28,33,43,.045) 1px, transparent 1px),
                    linear-gradient(90deg, rgba(28,33,43,.045) 1px, transparent 1px);
  background-size: 32px 32px; }
.page > * { position: relative; }

.kicker { font-family: "JetBrains Mono", monospace; font-size: 10.5px; letter-spacing: .16em;
  text-transform: uppercase; color: #8C8577; }
h1 { font-family: "Inter Display", "Inter", sans-serif; font-size: 44px; line-height: .98; font-weight: 800;
  letter-spacing: -.035em; margin: 8px 0 0; }
h1 em { font-style: normal; color: #B4512F; }
.lead { font-size: 13px; line-height: 1.45; color: #4A505E; margin: 10px 0 0; max-width: 620px; }
.lead a { color: #33567E; text-decoration: none; border-bottom: 1px solid rgba(51,86,126,.3); }

.grid { display: grid; grid-template-columns: repeat(6, 1fr); gap: 8px; margin-top: 14px; }
.card { background: #FFFDF7; border: 1.5px solid rgba(28,33,43,.1); border-radius: 18px;
  padding: 11px 14px 12px; grid-column: span 2; display: flex; flex-direction: column; }
.card.w3 { grid-column: span 3; }
.card.w4 { grid-column: span 4; }
.card.w6 { grid-column: span 6; }
.card.hero { background: #1C212B; border-color: #1C212B; }
.card .big { font-family: "Inter Display", "Inter", sans-serif; font-weight: 800; letter-spacing: -.035em;
  font-size: 40px; line-height: 1; color: var(--c, #1C212B); }
.card.w2 .big, .card:not(.w3):not(.w4):not(.w6) .big { font-size: 34px; }
.card.hero .big { font-size: 66px; color: #F2C14E; }
.card .label { font-size: 12px; line-height: 1.3; margin-top: 6px; color: #2A303C; font-weight: 600; }
.card.hero .label { color: #F7F4EC; font-size: 15px; }
.card .joke { font-size: 11px; line-height: 1.32; margin-top: 5px; color: #6B7280; }
.card.hero .joke { color: rgba(247,244,236,.72); font-size: 13px; }
.card .tag { font-family: "JetBrains Mono", monospace; font-size: 9px; letter-spacing: .1em;
  text-transform: uppercase; color: #A39B8B; margin-top: auto; padding-top: 6px; }
.card.hero .tag { color: rgba(247,244,236,.45); }

.bars { margin-top: 5px; display: flex; flex-direction: column; gap: 4px; }
.bars .row { display: grid; grid-template-columns: 92px 1fr 30px; align-items: center; gap: 8px; font-size: 11px; }
.bars .row u { text-decoration: none; font-family: "JetBrains Mono", monospace; color: #4A505E; }
.bars .row .track { height: 11px; background: rgba(28,33,43,.07); border-radius: 6px; overflow: hidden; }
.bars .row .fill { height: 100%; border-radius: 6px; background: var(--c, #33567E); }
.bars .row b { font-family: "JetBrains Mono", monospace; font-size: 11px; text-align: right; }

.foot { position: absolute; left: 44px; right: 44px; bottom: 18px; display: flex; justify-content: space-between;
  font-family: "JetBrains Mono", monospace; font-size: 9.5px; color: #A39B8B; }
"""


def card(big, label, joke, tag, color="#1C212B", width=2, hero=False):
    cls = "card" + (f" w{width}" if width != 2 else "") + (" hero" if hero else "")
    return (f'<div class="{cls}" style="--c:{color}"><div class="big">{big}</div>'
            f'<div class="label">{label}</div><div class="joke">{joke}</div>'
            f'<div class="tag">{tag}</div></div>')


GREEN, RED, BLUE, OCHRE, PLUM = "#5D7038", "#B4512F", "#33567E", "#B8860B", "#6B4C74"

page1_cards = [
    card(f"{ENT['item_frame']}", "рамок с предметами висит в мире",
         f"Это больше, чем все монстры вместе ({MONSTERS}). Ты не выживаешь, ты музей открыл.",
         "самый массовый объект", width=6, hero=True),
    card(f"{TOTAL_ENT}", "существ и объектов живёт в мире",
         "И ровно один игрок, который за всё это отвечает.", "34 разных типа", BLUE, 3),
    card(f"{FARM}", "кур, овец, коров и свиней",
         f"{ENT['chicken']} курей, {ENT['sheep']} овец, {ENT['cow']} коров, {ENT['pig']} свиней. Ферма победила выживание.",
         "мирное население", GREEN, 3),
    card(f"{ENT['item']}", "предметов валяется на земле",
         "Каждый тикает и ждёт, пока его подберут или он растворится.", "потери", OCHRE),
    card(f"{TILES}", "сундуков, печек и воронок",
         "Блок-сущности: их сервер пересчитывает каждый тик.", "block entities", PLUM),
    card(f"{ENT['chest_minecart']}", "вагонеток с сундуками",
         "Кто-то строит склад на рельсах.", "логистика", BLUE),
    card(f"{n(FIELDS, 0)}", "футбольных полей загружено в память",
         f"{CHUNKS} чанков = {n(AREA_M2)} м². Это {n(AREA_M2 / 10000, 1)} гектара, и всё ради одного человека.",
         "площадь мира в памяти", GREEN, 3),
    card(f"{n(BLOCKS / 1e6, 1)} млн", "блоков сервер держит в голове",
         "Каждый чанк - колонка от -64 до 320 по высоте. Вот куда уходит память.",
         "объём загруженного", BLUE, 3),
    card("20,00", "тиков в секунду, идеально",
         "Из 20 возможных. Сервер скучает.", "TPS за минуту", GREEN),
    card(f"{n(K(F['mspt_1m'], 'median'), 1)} мс", "длится обычный тик",
         "При бюджете 50 мс. Занято 11% времени.", "медиана", GREEN),
    card("647 мс", "длился самый долгий тик",
         "Один раз на старте сервер зевнул на две трети секунды и потерял 31 тик.", "рекорд", RED),
]

page2_cards = [
    card(f"{n(GC_PER_DAY)}", "раз в сутки сервер замирает, чтобы вынести мусор",
         f"Пауза {n(GC['avg_time'], 1)} мс каждые {n(GC['avg_freq'] / 1000, 1)} секунды. "
         f"За сутки набегает {n(GC_DAY_SEC / 60, 1)} минуты полной тишины, и это больше одного тика за раз.",
         "сборщик мусора G1", RED, 6),
    card(f"{n(CPU_SHARE, 1)}%", "процессора ноды принадлежит тебе",
         "2 потока из 48 на 24-ядерном EPYC. Остальное - соседи.", "доля CPU", OCHRE, 3),
    card(f"{n(RAM_SHARE, 1)}%", "памяти ноды выделено серверу",
         "2 ГБ из 251,6 ГиБ. Сервер живёт в чулане очень большого дома.", "доля RAM", OCHRE, 3),
    card("7 лет", "процессору, на котором всё это работает",
         "AMD EPYC 7402P вышел в 2019 году. Старше, чем половина мобов в 26.3.", "железо", PLUM),
    card(f"{n(TX_MONTH_GIB, 0)} ГиБ", "трафика в месяц при игре без остановки",
         f"34 КБ/с. Примерно {n(MOVIES, 0)} фильмов в 1080p.", "сеть", BLUE),
    card("4", "порта открыто ради одного игрока",
         "Игра, JSON-RPC, query и JMX-агент хостинга.", "19254 / 9900 / 9898 / 9899", PLUM),
    card(f"{n(DIST)}", "блоков от точки 0,0 до игрока",
         f"Пешком {n(WALK_MIN, 0)} минут без остановок. На элитрах - около минуты.", "прогулка", GREEN, 3),
    card("1 866", "достижений сервер загрузил при старте",
         "И ноль плагинов. Ванильный выживач в чистом виде.", "старт за 14,9 с", BLUE, 3),
    card("60 с", "и сервер засыпает без игроков",
         "pause-when-empty-seconds. Вышел покурить - мир встал.", "энергосбережение", OCHRE),
    card("3", "счастливых гаста в мире",
         f"А ещё {ENT['iron_golem']} железных голема на {ENT['villager']} жителей: по одному на четырёх.",
         "редкости", PLUM),
]

rare = [(k, v) for k, v in ENT.items() if v <= 3 and k != 'player']
rare_list = ", ".join(f"{k} x{v}" for k, v in rare[:7])

bars_src = [(k, v, RED if k in ('zombie', 'skeleton', 'creeper', 'enderman', 'spider', 'drowned', 'pillager',
                                'zombie_nautilus') else GREEN if k in (
    'chicken', 'sheep', 'cow', 'pig', 'bee', 'cat', 'wolf', 'parrot', 'rabbit', 'horse', 'armadillo', 'villager',
    'happy_ghast', 'iron_golem', 'glow_squid', 'squid', 'bat') else BLUE)
             for k, v in list(ENT.items())[:6]]
mx = max(v for _, v, _ in bars_src)
bars = ''.join(
    f'<div class="row" style="--c:{c}"><u>{E(k)}</u><div class="track">'
    f'<div class="fill" style="width:{v / mx * 100:.1f}%"></div></div><b>{v}</b></div>'
    for k, v, c in bars_src)

page1 = f"""
<section class="page">
  <div class="kicker">Досье на сервер · снято профилировщиком spark</div>
  <h1>Твой сервер<br>в <em>двадцати двух</em> фактах</h1>
  <p class="lead">Paper {E(F['platform']['mc'])} на бесплатном Aternos, снимок {E(F['generated_iso'])},
    один игрок онлайн. Все числа настоящие, вытащены из
    <a href="https://spark.lucko.me/ZYkc5dzN8D">профиля spark</a>. Шутки тоже настоящие.</p>
  <div class="grid">{''.join(page1_cards)}</div>
  <div class="foot"><span>Факты о сервере · Paper {E(F['platform']['mc'])}</span><span>стр. 1 / 2</span></div>
</section>
"""

page2 = f"""
<section class="page">
  <div class="kicker">Досье на сервер · часть 2</div>
  <h1 style="font-size:36px">Факты, которые<br>никто не заказывал</h1>
  <div class="grid">{''.join(page2_cards)}
    <div class="card w3"><div class="big" style="font-size:22px">Топ населения</div>
      <div class="bars">{bars}</div>
      <div class="tag">6 самых массовых из 34 типов</div></div>
    <div class="card w3"><div class="big" style="--c:{PLUM};font-size:34px">{len(rare)}</div>
      <div class="label">типов существ представлены в единичных экземплярах</div>
      <div class="joke">{E(rare_list)}. И каждый из них честно тикает.</div>
      <div class="tag">редкости мира</div></div>
  </div>
  <div class="foot"><span>Данные: spark.lucko.me/ZYkc5dzN8D</span><span>стр. 2 / 2</span></div>
</section>
"""

doc = f"""<!doctype html><html lang="ru"><head><meta charset="utf-8">
<title>Твой сервер в фактах - профиль spark</title><style>{CSS}</style></head>
<body>{page1}{page2}</body></html>"""
open(os.path.join(HERE, 'facts.html'), 'w', encoding='utf-8').write(doc)
print('готово: facts.html')
