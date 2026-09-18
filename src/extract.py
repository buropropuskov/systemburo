import struct, json, collections
exec(open('dump.py').read().split('data = open')[0])

data = open('spark.bin','rb').read()

def tree(b, depth=0, maxdepth=14):
    """Разбор в вложенные dict/list по номерам полей."""
    out = collections.defaultdict(list)
    i, n = 0, len(b)
    while i < n:
        try:
            key, i = varint(b, i)
        except Exception:
            break
        f, wt = key >> 3, key & 7
        if f == 0 or f > 512: break
        try:
            if wt == 0:
                v, i = varint(b, i); out[f].append(v)
            elif wt == 1:
                v = struct.unpack('<d', b[i:i+8])[0]; i += 8; out[f].append(v)
            elif wt == 2:
                ln, i = varint(b, i)
                if ln > n - i: break
                chunk = b[i:i+ln]; i += ln
                s = None
                try:
                    t = chunk.decode('utf-8')
                    if t and all(c.isprintable() or c in '\n\t ' for c in t): s = t
                except Exception: pass
                sub = tree(chunk, depth+1, maxdepth) if depth < maxdepth else {}
                if s is not None and sub:
                    val = dict(sub); val['_s'] = [s]
                elif s is not None:
                    val = s
                else:
                    val = sub
                out[f].append(val)
            elif wt == 5:
                v = struct.unpack('<f', b[i:i+4])[0]; i += 4; out[f].append(v)
            else: break
        except Exception:
            break
    return dict(out)

def as_str(v):
    if isinstance(v, str): return v
    if isinstance(v, dict):
        x = v.get('_s')
        if isinstance(x, list) and x and isinstance(x[0], str): return x[0]
    return None


t = tree(data)
meta = t[1][0]
plat_stats = meta[3][0]
sys_stats = meta[4][0]

def g(d, *path, default=None):
    cur = d
    for p in path:
        if not isinstance(cur, dict) or p not in cur: return default
        cur = cur[p][0]
    return as_str(cur) or cur

facts = {}
facts['platform'] = {'brand': g(meta,2,2), 'version': g(meta,2,3), 'mc': g(meta,2,4)}
facts['user'] = g(meta,1,2)
facts['generated_ms'] = meta[5][0] if 5 in meta else None
facts['uptime_ms'] = plat_stats[3][0] if 3 in plat_stats else None
facts['players'] = plat_stats[7][0] if 7 in plat_stats else None
tps = plat_stats[4][0]
facts['tps'] = {k: v[0] for k, v in tps.items()}
mspt = plat_stats[5][0]
def roll(d):
    d = d or {}
    return {'mean': d.get(1,[None])[0], 'max': d.get(2,[None])[0], 'min': d.get(3,[None])[0],
            'median': d.get(4,[None])[0], 'p95': d.get(5,[None])[0]}
facts['mspt_1m'] = roll(mspt[1][0]); facts['mspt_5m'] = roll(mspt[2][0])
facts['ping'] = plat_stats.get(6)
# gc
facts['gc'] = []
for e in plat_stats.get(2, []):
    if isinstance(e, dict):
        st = e.get(2,[{}])[0] or {}
        facts['gc'].append({'name': as_str(e.get(1,[None])[0]), 'total': st.get(1,[None])[0],
                            'avg_time': st.get(2,[None])[0], 'avg_freq': st.get(3,[None])[0]})
# world
w = plat_stats.get(8,[{}])[0]
facts['total_entities'] = w.get(1,[None])[0]
ents = {}
for e in w.get(2, []):
    if isinstance(e, dict):
        nm = as_str(e.get(1,[None])[0]); ct = e.get(2,[None])[0]
        if nm and isinstance(ct, int): ents[nm] = ct
facts['entities'] = dict(sorted(ents.items(), key=lambda kv: -kv[1]))
facts['worlds_raw_keys'] = sorted(w.keys())
# system
cpu = sys_stats[1][0]
facts['cpu'] = {'threads': cpu.get(1,[None])[0], 'model': as_str(cpu.get(4,[None])[0]),
                'proc_usage': roll(cpu.get(2,[{}])[0]), 'sys_usage': roll(cpu.get(3,[{}])[0])}
mem = sys_stats[2][0]
facts['mem'] = {'phys_used': (mem.get(1,[{}])[0] or {}).get(1,[None])[0], 'phys_total': (mem.get(1,[{}])[0] or {}).get(2,[None])[0],
                'swap_used': (mem.get(2,[{}])[0] or {}).get(1,[None])[0], 'swap_total': (mem.get(2,[{}])[0] or {}).get(2,[None])[0]}
disk = sys_stats[4][0]
facts['disk'] = {'used': disk.get(1,[None])[0], 'total': disk.get(2,[None])[0]}
facts['os'] = {'arch': g(sys_stats,5,1), 'name': g(sys_stats,5,2), 'kernel': g(sys_stats,5,3)}
facts['java'] = {'vendor': g(sys_stats,6,1), 'version': g(sys_stats,6,2), 'vendor_version': g(sys_stats,6,3),
                 'flags': g(sys_stats,6,4)}
facts['sys_uptime_ms'] = sys_stats.get(7,[None])[0]
facts['jvm'] = {'name': g(sys_stats,9,1), 'vendor': g(sys_stats,9,2), 'version': g(sys_stats,9,3)}
net = {}
for e in sys_stats.get(8, []):
    if isinstance(e, dict):
        nm = as_str(e.get(1,[None])[0]); st = e.get(2,[{}])[0] or {}
        net[nm] = {k: roll(v[0]) if isinstance(v[0], dict) else v[0] for k, v in st.items()}
facts['net'] = net
# окна
facts['windows'] = []
for wnd in t.get(2, []):
    if isinstance(wnd, dict):
        s = wnd.get(2,[{}])[0]
        facts['windows'].append({k: (v[0] if not isinstance(v[0], dict) else v[0]) for k, v in s.items()})
facts['block3'] = {k: (v[0] if not isinstance(v[0], dict) else list(v[0].keys())) for k, v in t.get(3,[{}])[0].items()} if 3 in t else None
# server.properties
cfgs = {}
for e in meta.get(6, []):
    if isinstance(e, dict):
        k = e.get(1,[None])[0]; v = e.get(2,[None])[0]
        if isinstance(k, str) and isinstance(v, str): cfgs[k] = v
facts['configs'] = list(cfgs.keys())
props = json.loads(cfgs.get('server.properties','{}')) if cfgs.get('server.properties') else {}
facts['props'] = props
json.dump(facts, open('facts.json','w'), ensure_ascii=False, indent=1, default=str)
print('ключи:', list(facts))
print('TPS:', facts['tps'])
print('MSPT 1м:', facts['mspt_1m'])
print('MSPT 5м:', facts['mspt_5m'])
print('CPU:', facts['cpu'])
print('сущностей всего:', facts['total_entities'], '| типов:', len(facts['entities']))
print('топ мобов:', list(facts['entities'].items())[:12])
print('окна:', facts['windows'])
print('gc:', facts['gc'])
print('net:', json.dumps(facts['net'], ensure_ascii=False)[:300])
print('block3:', facts['block3'])
print('worlds keys:', facts['worlds_raw_keys'])
