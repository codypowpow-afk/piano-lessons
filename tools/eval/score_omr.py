"""Score OMR output against a reference note list by sequence alignment.

usage: score_omr.py songs.json piece_index ref.json [--detail]
Per hand, notes are ordered by (time, pitch) and aligned with Needleman-Wunsch
(match +2, substitution -1, gap -1). Substitutions are classed by interval.
"""
import json, sys
from collections import Counter

NAMES = ['C', 'Db', 'D', 'Eb', 'E', 'F', 'Gb', 'G', 'Ab', 'A', 'Bb', 'B']


def nm(p):
    return NAMES[p % 12] + str(p // 12 - 1)


def align(a, b):
    n, m = len(a), len(b)
    # banded DP would be faster; n,m < 2000 is fine
    INF = -10**9
    score = [[0] * (m + 1) for _ in range(n + 1)]
    back = [[0] * (m + 1) for _ in range(n + 1)]
    for i in range(1, n + 1):
        score[i][0] = -i
        back[i][0] = 1
    for j in range(1, m + 1):
        score[0][j] = -j
        back[0][j] = 2
    for i in range(1, n + 1):
        ai = a[i - 1]
        si = score[i]
        sp = score[i - 1]
        bi = back[i]
        for j in range(1, m + 1):
            d = sp[j - 1] + (2 if ai == b[j - 1] else -1)
            u = sp[j] - 1
            l = si[j - 1] - 1
            if d >= u and d >= l:
                si[j] = d; bi[j] = 0
            elif u >= l:
                si[j] = u; bi[j] = 1
            else:
                si[j] = l; bi[j] = 2
    ops = []
    i, j = n, m
    while i > 0 or j > 0:
        k = back[i][j]
        if i > 0 and j > 0 and k == 0:
            ops.append(('M' if a[i - 1] == b[j - 1] else 'S', i - 1, j - 1)); i -= 1; j -= 1
        elif i > 0 and (j == 0 or k == 1):
            ops.append(('I', i - 1, None)); i -= 1   # in OMR only: false note
        else:
            ops.append(('D', None, j - 1)); j -= 1   # in ref only: missed note
    return ops[::-1]


def evaluate(omr_notes, ref_notes, detail=False):
    res = {}
    tot = Counter()
    for h in (0, 1):
        a = sorted([n for n in omr_notes if n['h'] == h], key=lambda n: (round(n['s'], 3), n['p']))
        b = sorted([n for n in ref_notes if n['h'] == h], key=lambda n: (round(n['s'], 3), 0 if n.get('g') else 1, n['p']))
        ops = align([n['p'] for n in a], [n['p'] for n in b])
        c = Counter()
        subs = Counter()
        for op, i, j in ops:
            c[op] += 1
            if op == 'S':
                iv = abs(a[i]['p'] - b[j]['p'])
                cls = 'semitone(accidental)' if iv in (1, 2) and (a[i]['p'] % 12 != b[j]['p'] % 12) and iv == 1 else ('octave' if iv % 12 == 0 else ('step' if iv <= 2 else 'other'))
                subs[cls] += 1
        res[h] = (c, subs, ops, a, b)
        tot += c
    return res, tot


def report(res, tot):
    for h in (0, 1):
        c, subs, ops, a, b = res[h]
        n_ref = c['M'] + c['S'] + c['D']
        n_omr = c['M'] + c['S'] + c['I']
        print(f"{'RH' if h == 0 else 'LH'}: ref {n_ref} omr {n_omr} | correct {c['M']} ({100*c['M']/max(1,n_ref):.1f}% of ref) wrong-pitch {c['S']} missed {c['D']} extra {c['I']} | {dict(subs)}")
    n_ref = tot['M'] + tot['S'] + tot['D']
    n_omr = tot['M'] + tot['S'] + tot['I']
    p = tot['M'] / max(1, n_omr)
    r = tot['M'] / max(1, n_ref)
    f = 2 * p * r / max(1e-9, p + r)
    print(f"ALL: precision {100*p:.1f}%  recall {100*r:.1f}%  F1 {100*f:.1f}%")
    return f


if __name__ == '__main__':
    songs = json.load(open(sys.argv[1]))
    s = songs[int(sys.argv[2])]
    omr = [{'p': n['p'], 's': n['s'], 'h': n.get('h', 0)} for n in s['notes']]
    ref = json.load(open(sys.argv[3]))
    for a in sys.argv:
        if a.startswith('--until='):
            lim = float(a.split('=')[1])
            ref = [r for r in ref if r['s'] < lim]
            # OMR: keep notes up to the same bar (6 quarters per 12/8 bar, OMR uses 4 beats per bar)
            bars = lim / 6
            omr = [n for n, src in zip(omr, s['notes']) if src['m'] < bars + 1]
    res, tot = evaluate(omr, ref)
    report(res, tot)
    if '--detail' in sys.argv:
        for h in (0, 1):
            c, subs, ops, a, b = res[h]
            print('---', 'RH' if h == 0 else 'LH')
            for op, i, j in ops[:400]:
                if op == 'M':
                    continue
                ai = f"{nm(a[i]['p'])}@m{a[i].get('m', '?')}" if i is not None else '-'
                bj = nm(b[j]['p']) if j is not None else '-'
                print(op, 'omr', ai, 'ref', bj, 't_ref', round(b[j]['s'] / 6, 1) if j is not None else '')
