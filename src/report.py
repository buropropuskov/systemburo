# -*- coding: utf-8 -*-
"""Отчёт по spark-профилю Minecraft-сервера: HTML под печать в PDF (A4)."""
import json, os, html

HERE = os.path.dirname(os.path.abspath(__file__))
F = json.load(open(os.path.join(HERE, 'facts.json'), encoding='utf-8'))
E = html.escape
PROFILE_URL = "https://spark.lucko.me/ZYkc5dzN8D"

def K(d, k):
    return d.get(str(k), d.get(k))


gib = lambda b: f"{b / 1024**3:.1f} ГиБ"
kb = lambda b: f"{b / 1024:.1f} КБ/с"


def num(x, d=2):
    s = f"{x:,.{d}f}".replace(',', ' ').replace('.', ',')
    return s


CSS = """
@page { size: A4; margin: 0; }
* { box-sizing: border-box; }
html, body { margin: 0; background: #E7E3D9; }
body { font-family: "Inter", sans-serif; color: #1C212B; -webkit-font-smoothing: antialiased; }
.page {
  position: relative; width: 794px; height: 1123px; padding: 40px 50px 38px;
  background: #F7F4EC; overflow: hidden; page-break-after: always; break-after: page;
}
.page:last-child { page-break-after: auto; break-after: auto; }
.page::before {
  content: ""; position: absolute; inset: 0; pointer-events: none;
  background-image: linear-gradient(rgba(28,33,43,.04) 1px, transparent 1px),
                    linear-gradient(90deg, rgba(28,33,43,.04) 1px, transparent 1px);
  background-size: 34px 34px;
}
.page > * { position: relative; }
.kicker { font-family: "JetBrains Mono", monospace; font-size: 11px; letter-spacing: .14em;
  text-transform: uppercase; color: #8C8577; }
h1 { font-family: "Inter Display", "Inter", sans-serif; font-size: 34px; line-height: 1.04;
  font-weight: 800; letter-spacing: -.03em; margin: 10px 0 0; }
h1 em { font-style: normal; color: #B4512F; }
.subtitle { font-size: 13px; color: #4A505E; margin: 10px 0 0; line-height: 1.4; }
.subtitle a { color: #33567E; text-decoration: none; border-bottom: 1px solid rgba(51,86,126,.35); }
h2 { font-size: 17px; font-weight: 700; letter-spacing: -.01em; margin: 18px 0 9px;
  display: flex; align-items: baseline; gap: 10px; }
h2 u { text-decoration: none; font-family: "JetBrains Mono", monospace; font-size: 11px;
  color: #A39B8B; letter-spacing: .1em; text-transform: uppercase; }

.kpi { display: grid; grid-template-columns: repeat(3, 1fr); gap: 9px; margin-top: 16px; }
.kpi .cell { background: #FFFDF7; border: 1.5px solid rgba(28,33,43,.1); border-radius: 16px;
  padding: 11px 14px; }
.kpi .cell b { display: block; font-family: "Inter Display", "Inter", sans-serif;
  font-size: 24px; font-weight: 800; letter-spacing: -.02em; line-height: 1.1; color: var(--c, #1C212B); }
.kpi .cell span { display: block; font-size: 12px; color: #6B7280; margin-top: 4px; line-height: 1.3; }
.kpi .cell i { font-style: normal; font-family: "JetBrains Mono", monospace; font-size: 11px; color: #A39B8B; }

table { width: 100%; border-collapse: collapse; font-size: 12.5px; }
th { text-align: left; font-family: "JetBrains Mono", monospace; font-size: 10px; letter-spacing: .1em;
  text-transform: uppercase; color: #8C8577; font-weight: 500; padding: 0 10px 7px 0;
  border-bottom: 1.5px solid rgba(28,33,43,.14); }
td { padding: 5.5px 10px 5.5px 0; border-bottom: 1px solid rgba(28,33,43,.07); vertical-align: top; line-height: 1.35; }
td.mono, th.mono { font-family: "JetBrains Mono", monospace; }
td.you { color: #5D7038; font-weight: 600; }
td.warn { color: #B4512F; font-weight: 600; }

.note { background: rgba(28,33,43,.045); border-radius: 13px; padding: 10px 14px; font-size: 12.5px;
  line-height: 1.4; margin-top: 10px; }
.note b { font-family: "JetBrains Mono", monospace; font-size: 10px; letter-spacing: .1em;
  text-transform: uppercase; color: #8C8577; font-weight: 500; display: block; margin-bottom: 5px; }
.note.alarm { background: rgba(180,81,47,.09); }
.note.alarm b { color: #B4512F; }

.bars { display: flex; flex-direction: column; gap: 4px; margin-top: 8px; }
.bars .row { display: grid; grid-template-columns: 96px 1fr 34px; align-items: center; gap: 9px;
  font-size: 11.5px; }
.bars .row u { text-decoration: none; font-family: "JetBrains Mono", monospace; color: #4A505E; }
.bars .row .track { height: 12px; background: rgba(28,33,43,.06); border-radius: 7px; overflow: hidden; }
.bars .row .fill { height: 100%; border-radius: 7px; background: var(--c, #33567E); }
.bars .row b { font-family: "JetBrains Mono", monospace; font-size: 12px; text-align: right; font-weight: 600; }

.two { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; }
.verdict { display: flex; flex-direction: column; gap: 7px; margin-top: 10px; }
.verdict .item { display: flex; gap: 10px; font-size: 12.5px; line-height: 1.38; }
.verdict .item s { text-decoration: none; flex: 0 0 22px; height: 22px; border-radius: 999px;
  background: var(--c, #33567E); color: #F7F4EC; font-size: 11px; font-weight: 700;
  display: flex; align-items: center; justify-content: center; font-family: "JetBrains Mono", monospace; }
.foot { position: absolute; left: 52px; right: 52px; bottom: 22px; display: flex;
  justify-content: space-between; font-family: "JetBrains Mono", monospace; font-size: 10px; color: #A39B8B; }
"""

w_new, w_mid, w_old = F['windows'][0], F['windows'][1], F['windows'][2]
props = F['props']
ent = F['entities']
top_ent = list(ent.items())[:11]
max_ent = max(c for _, c in top_ent)
eth = F['net']['eth0']
tx_month = eth['2']['mean'] * 86400 * 30 / 1024**3

kpi = [
    ("#5D7038", f"{num(K(F['tps'], 1), 2)}", "TPS за минуту", "предел 20,00"),
    ("#5D7038", f"{num(F['mspt_1m']['median'], 2)} мс", "MSPT, медиана тика", "бюджет 50 мс"),
    ("#B8860B", f"{num(F['mspt_1m']['p95'], 2)} мс", "MSPT, 95-й процентиль", "худшие тики"),
    ("#33567E", f"{F['total_entities']}", "сущностей в мире", f"{len(ent)} типов"),
    ("#33567E", f"{K(w_new, 10)}", "загруженных чанков", f"{K(w_new, 9)} блок-сущностей"),
    ("#B4512F", f"{num(F['mspt_5m']['max'], 0)} мс", "худший тик за 5 минут", "старт сервера"),
]

hw_rows = [
    ("Процессор", F['cpu']['model'] + " (24 ядра / 48 потоков)", "2 потока", "availableProcessors"),
    ("Оперативная память", f"{gib(F['mem']['phys_total'])} на ноде, занято {gib(F['mem']['phys_used'])}",
     "2070 МБ heap", "-Xmx2070M -Xms1035M"),
    ("Диск", f"{gib(F['disk']['total'])} на ноде, занято {gib(F['disk']['used'])}", "квота не видна", "spark читает ФС ноды"),
    ("Операционная система", f"{F['os']['name']}, ядро {F['os']['kernel']}, {F['os']['arch']}", "-", "контейнер"),
    ("Java", f"{F['java']['vendor_version']} ({F['jvm']['name']})", "-", "JDK 25, LTS"),
]

win_rows = []
for label, w in (("окно 1 (свежее)", w_new), ("окно 2", w_mid), ("окно 3 (старт)", w_old)):
    win_rows.append((label, K(w, 1), num(K(w, 4), 2), num(K(w, 5), 2), num(K(w, 6), 2), f"{K(w, 2)*100:.1f}".replace('.', ','), K(w, 8)))

gc = [g for g in F['gc'] if g.get('total')]

page1 = f"""
<section class="page">
  <div class="kicker">Отчёт по производительности · Minecraft-сервер</div>
  <h1>Что <em>spark</em> рассказал<br>о сервере</h1>
  <p class="subtitle">Health-снимок профилировщика spark, снят {E(F['generated_iso'])} на {E(F['platform']['brand'])}
    {E(F['platform']['version'])} (Minecraft {E(F['platform']['mc'])}), хостинг Aternos.
    Источник: <a href="{PROFILE_URL}">{PROFILE_URL}</a>. Плагинов на сервере нет, снимок сделан из консоли.</p>

  <div class="kpi">
    {''.join(f'<div class="cell" style="--c:{c}"><b>{v}</b><span>{t}</span><i>{h}</i></div>' for c, v, t, h in kpi)}
  </div>

  <h2>Железо ноды и твоя доля <u>система</u></h2>
  <table>
    <tr><th>Ресурс</th><th>Физическая нода</th><th>Доступно серверу</th><th class="mono">откуда</th></tr>
    {''.join(f'<tr><td>{E(a)}</td><td>{E(b)}</td><td class="{"warn" if "не видна" in c else "you"}">{E(c)}</td><td class="mono" style="color:#A39B8B">{E(d)}</td></tr>' for a, b, c, d in hw_rows)}
  </table>
  <div class="note"><b>вывод по железу</b>
    Сервер живёт в контейнере на 24-ядерном EPYC, но видит только два потока. Paper это подтверждает
    в логе: один worker-поток и один I/O-поток на работу с чанками. Значит генерация новой территории
    упирается в одно ядро, а не в память.</div>

  <h2>Тики: три окна по минуте <u>tps / mspt</u></h2>
  <table>
    <tr><th>Окно</th><th class="mono">тиков</th><th class="mono">TPS</th><th class="mono">тик, мс</th>
      <th class="mono">худший тик, мс</th><th class="mono">CPU, %</th><th class="mono">сущностей</th></tr>
    {''.join(f'<tr><td>{E(a)}</td><td class="mono">{b}</td><td class="mono">{c}</td><td class="mono">{d}</td><td class="mono {"warn" if float(e.replace(" ","").replace(",","."))>100 else ""}">{e}</td><td class="mono">{f_}</td><td class="mono">{g}</td></tr>' for a, b, c, d, e, f_, g in win_rows)}
  </table>
  <div class="note alarm"><b>единственный настоящий провал</b>
    В самом раннем окне один тик длился {num(F['mspt_5m']['max'], 0)} мс вместо 50 и сервер потерял 31 тик
    из 1200. Это момент старта: прогрев мира и вход игрока, когда с диска поднимаются чанки.
    В следующие две минуты медиана тика держится около {num(F['mspt_1m']['median'], 1)} мс,
    то есть сервер тратит примерно десятую часть отведённого бюджета.</div>

  <div class="foot"><span>Профиль spark · {E(F['platform']['brand'])} {E(F['platform']['mc'])}</span><span>стр. 1 / 2</span></div>
</section>
"""

bars = ''.join(
    f'<div class="row" style="--c:{"#B4512F" if k in ("zombie","skeleton","creeper","drowned","pillager") else "#5D7038" if k in ("chicken","sheep","cow","pig","bee","cat","rabbit","villager","armadillo","glow_squid","bat") else "#33567E"}">'
    f'<u>{E(k)}</u><div class="track"><div class="fill" style="width:{c / max_ent * 100:.1f}%"></div></div><b>{c}</b></div>'
    for k, c in top_ent
)

cfg_rows = [
    ("view-distance", props['view-distance'], "чанков вокруг игрока отправляется клиенту"),
    ("simulation-distance", props['simulation-distance'], "радиус, где мобы и механизмы реально тикают"),
    ("max-players", props['max-players'], "слотов заявлено"),
    ("online-mode", str(props['online-mode']).lower(), "проверка аккаунта отключена"),
    ("difficulty", props['difficulty'], "сложность мира"),
    ("pause-when-empty-seconds", props['pause-when-empty-seconds'], "через сколько сервер замирает без игроков"),
    ("network-compression-threshold", props['network-compression-threshold'], "байт до сжатия пакета"),
    ("spawn-protection", props['spawn-protection'], "защита спавна выключена"),
]

verdict = [
    ("#5D7038", "1", f"Сервер здоров. TPS {num(K(F['tps'], 1), 2)} из 20, "
                     f"медиана тика {num(F['mspt_1m']['median'], 1)} мс при бюджете 50 мс. Запас примерно десятикратный, "
                     "но проверен он на одном игроке."),
    ("#B8860B", "2", f"Паузы сборщика мусора крупнее тика: G1 останавливает мир в среднем на "
                     f"{num(gc[0]['avg_time'], 1)} мс каждые {num(gc[0]['avg_freq'] / 1000, 1)} с. "
                     "Один тик при этом теряется целиком. Лечится добавлением памяти и ядер, а не настройками игры."),
    ("#B4512F", "3", "Режим offline-mode оставляет вход под любым ником. На бесплатном хостинге со случайным "
                     "адресом это терпимо, на платном с постоянным адресом любой сможет зайти твоим ником."),
    ("#33567E", "4", f"Трафик крошечный: исходящих {kb(eth['2']['mean'])} в среднем и {kb(eth['2']['max'])} на пике, "
                     f"что даёт около {num(tx_month, 1)} ГиБ в месяц при работе без остановки. "
                     "Для любого хостинга это ничто, лимит трафика можно не смотреть."),
    ("#33567E", "5", f"Мир пока лёгкий: {F['total_entities']} сущностей и {K(w_new, 10)} чанков, "
                     f"из них {K(w_new, 9)} блок-сущностей вроде сундуков и печей. Половина живности - животные "
                     "и брошенные предметы, монстров немного."),
]

page2 = f"""
<section class="page">
  <div class="kicker">Отчёт по производительности · часть 2</div>
  <h1>Мир, память и сеть</h1>

  <div class="two">
    <div>
      <h2>Кто населяет мир <u>топ-11 из {len(ent)}</u></h2>
      <div class="bars">{bars}</div>
      <div class="note"><b>всего</b>{F['total_entities']} сущностей.
        Зелёным - мирные, красным - монстры, синим - предметы и техника.</div>
    </div>
    <div>
      <h2>Сборка мусора <u>g1</u></h2>
      <table>
        <tr><th>Сборщик</th><th class="mono">раз</th><th class="mono">пауза</th><th class="mono">интервал</th></tr>
        {''.join(f'<tr><td>{E(g["name"])}</td><td class="mono">{g["total"]}</td><td class="mono {"warn" if g["avg_time"] > 50 else ""}">{num(g["avg_time"], 1)} мс</td><td class="mono">{num(g["avg_freq"] / 1000, 1)} с</td></tr>' for g in gc)}
      </table>

      <h2>Сеть <u>eth0</u></h2>
      <table>
        <tr><th>Направление</th><th class="mono">среднее</th><th class="mono">пик</th><th class="mono">пакетов/с</th></tr>
        <tr><td>Приём</td><td class="mono">{kb(eth['1']['mean'])}</td><td class="mono">{kb(eth['1']['max'])}</td><td class="mono">{num(eth['3']['mean'], 0)}</td></tr>
        <tr><td>Отдача</td><td class="mono">{kb(eth['2']['mean'])}</td><td class="mono">{kb(eth['2']['max'])}</td><td class="mono">{num(eth['4']['mean'], 0)}</td></tr>
      </table>
      <div class="note"><b>в месяц</b>около {num(tx_month, 1)} ГиБ исходящего трафика при круглосуточной работе
        и одном игроке онлайн.</div>
    </div>
  </div>

  <h2>Настройки, которые влияют на нагрузку <u>server.properties</u></h2>
  <table>
    <tr><th class="mono">параметр</th><th class="mono">значение</th><th>что это значит</th></tr>
    {''.join(f'<tr><td class="mono">{E(k)}</td><td class="mono {"warn" if k == "online-mode" else ""}">{E(str(v))}</td><td>{E(d)}</td></tr>' for k, v, d in cfg_rows)}
  </table>

  <h2>Пять выводов <u>итог</u></h2>
  <div class="verdict">
    {''.join(f'<div class="item"><s style="--c:{c}">{n}</s><span>{E(t)}</span></div>' for c, n, t in verdict)}
  </div>

  <div class="note"><b>чего в снимке нет</b>
    Это health-отчёт, в нём нет стек-трейсов: увидеть, какой именно код ест тик, можно только полноценным
    профилем через «spark profiler start», а затем «spark profiler stop».</div>

  <div class="foot"><span>Данные: {PROFILE_URL}</span><span>стр. 2 / 2</span></div>
</section>
"""

doc = f"""<!doctype html><html lang="ru"><head><meta charset="utf-8">
<title>Отчёт по серверу Minecraft - профиль spark</title><style>{CSS}</style></head>
<body>{page1}{page2}</body></html>"""

out = os.path.join(HERE, 'report.html')
open(out, 'w', encoding='utf-8').write(doc)
print('готово:', out)
