import struct, collections

def varint(b, i):
    r = s = 0
    while True:
        if i >= len(b): raise ValueError
        x = b[i]; i += 1
        r |= (x & 0x7f) << s
        if not x & 0x80: return r, i
        s += 7
        if s > 70: raise ValueError

def walk(b, path, depth, sink, maxdepth):
    i, n = 0, len(b)
    while i < n:
        try:
            key, i = varint(b, i)
            f, wt = key >> 3, key & 7
            if f == 0 or f > 512: return
            p = f"{path}.{f}" if path else str(f)
            if wt == 0:
                v, i = varint(b, i); sink(p, 'int', v, depth)
            elif wt == 1:
                if i + 8 > n: return
                sink(p, 'f64', struct.unpack('<d', b[i:i+8])[0], depth); i += 8
            elif wt == 2:
                ln, i = varint(b, i)
                if ln > n - i: return
                chunk = b[i:i+ln]; i += ln
                s = None
                try:
                    t = chunk.decode('utf-8')
                    if t and all(c.isprintable() or c in '\n\t ' for c in t): s = t
                except Exception: pass
                if s is not None: sink(p, 'str', s, depth)
                if depth < maxdepth:
                    buf = []
                    try: walk(chunk, p, depth + 1, lambda *a: buf.append(a), maxdepth)
                    except Exception: buf = []
                    for a in buf: sink(*a)
            elif wt == 5:
                if i + 4 > n: return
                sink(p, 'f32', struct.unpack('<f', b[i:i+4])[0], depth); i += 4
            else: return
        except Exception:
            return

data = open('spark.bin', 'rb').read()
rows = []
walk(data, "", 0, lambda p, t, v, d: rows.append((p, t, v)), 9)
paths = collections.Counter(p for p, t, v in rows)
print("=== карта путей (до глубины 4) ===")
for p, c in sorted(paths.items()):
    if p.count('.') <= 3:
        sample = next((v for pp, t, v in rows if pp == p), None)
        s = str(sample)
        if len(s) > 70: s = s[:70] + '…'
        print(f"{p:16} x{c:<6} {s}")
