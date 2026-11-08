"""Minimal LilyPond -> note list converter for the Mutopia Op. 9 sources.

Handles \\relative, English note names, chords, durations, dots, ties, rests,
spacers, \\times, grace notes, << { } \\\\ { } >> voices, \\partial, and skips
markup/scheme/overrides. Output: JSON list of {p, s, d, h} with s/d in quarter
notes, h 0 = upper staff (right hand), 1 = lower (left hand).
"""
import json, re, sys

STEP = {'c': 0, 'd': 2, 'e': 4, 'f': 5, 'g': 7, 'a': 9, 'b': 11}
DIA = {'c': 0, 'd': 1, 'e': 2, 'f': 3, 'g': 4, 'a': 5, 'b': 6}
ACC = {'': 0, 's': 1, 'ss': 2, 'x': 2, 'f': -1, 'ff': -2, 'sharp': 1, 'flat': -1}
NOTE_RE_EN = re.compile(r"([a-g])(ss|s|ff|f|x)?([',]*)([!?]?)(\d+\.*)?")
NOTE_RE_NL = re.compile(r"([a-g])(isis|eses|ases|is|es|as|s)?([',]*)([!?]?)(\d+\.*)?")
NOTE_RE = NOTE_RE_EN
ACC_NL = {'': 0, 'is': 1, 'isis': 2, 'es': -1, 'eses': -2, 's': -1, 'as': -1, 'ases': -2}
LANG = ['en']
DUR_RE = re.compile(r"\s*(\d+)(\.*)")


def strip_comments(src):
    out = []
    for line in src.split('\n'):
        # keep % inside strings rare; good enough
        i = line.find('%')
        out.append(line if i < 0 else line[:i])
    return '\n'.join(out)


VARS = {}


class Parser:
    def __init__(self, src, hand):
        self.s = src
        self.i = 0
        self.hand = hand
        self.events = []
        self.t = 0.0          # quarter notes
        self.dur = 1.0        # quarter notes
        self.scale = 1.0
        self.grace = False
        self.ref = None       # (diatonic absolute, midi) for \relative
        self.tied = {}        # pitch -> event index
        self.partial = None

    def peek(self):
        return self.s[self.i] if self.i < len(self.s) else ''

    def ws(self):
        while self.i < len(self.s) and self.s[self.i].isspace():
            self.i += 1

    def skip_balanced(self, open_c, close_c):
        depth = 0
        while self.i < len(self.s):
            c = self.s[self.i]
            if c == '"':
                self.skip_string()
                continue
            self.i += 1
            if c == open_c:
                depth += 1
            elif c == close_c:
                depth -= 1
                if depth == 0:
                    return

    def skip_string(self):
        self.i += 1
        while self.i < len(self.s) and self.s[self.i] != '"':
            if self.s[self.i] == '\\':
                self.i += 1
            self.i += 1
        self.i += 1

    def skip_scheme(self):
        # at '#'
        while self.peek() == '#':
            self.i += 1
        if self.peek() == "'":
            self.i += 1
        c = self.peek()
        if c == '(':
            self.skip_balanced('(', ')')
        elif c == '"':
            self.skip_string()
        else:
            while self.i < len(self.s) and not self.s[self.i].isspace() and self.s[self.i] not in '{}<>':
                self.i += 1

    def skip_markup(self):
        self.ws()
        # markup argument: either {...} or a chain of \commands ending in a word/{...}
        while True:
            self.ws()
            c = self.peek()
            if c == '{':
                self.skip_balanced('{', '}')
                return
            if c == '"':
                self.skip_string()
                return
            if c == '#':
                self.skip_scheme()
                continue
            if c == '\\':
                self.word()
                continue
            # a bare word
            self.word_plain()
            return

    def word(self):
        # at '\\'
        j = self.i + 1
        while j < len(self.s) and (self.s[j].isalpha() or self.s[j] in '-'):
            j += 1
        w = self.s[self.i + 1:j]
        self.i = j
        return w

    def word_plain(self):
        j = self.i
        while j < len(self.s) and not self.s[j].isspace() and self.s[j] not in '{}<>()[]':
            j += 1
        w = self.s[self.i:j]
        self.i = j
        return w

    def read_duration(self):
        m = DUR_RE.match(self.s, self.i)
        if m and not self.s[self.i:self.i + 1].isalpha():
            n = int(m.group(1))
            dots = len(m.group(2))
            d = 4.0 / n
            add = d
            for _ in range(dots):
                add /= 2
                d += add
            self.i = m.end()
            return d
        return None

    def abs_pitch(self, letter, acc, octs):
        """Resolve a pitch in \\relative mode. Returns midi and diatonic."""
        dia_step = DIA[letter]
        if self.ref is None:
            # absolute: c = C3 (midi 48)
            octave = 3 + octs.count("'") - octs.count(',')
            dia = octave * 7 + dia_step
        else:
            rdia = self.ref
            # nearest diatonic position to reference
            base = (rdia // 7) * 7 + dia_step
            cands = [base - 7, base, base + 7]
            dia = min(cands, key=lambda d: abs(d - rdia))
            dia += 7 * (octs.count("'") - octs.count(','))
        octave = dia // 7
        alter = ACC_NL[acc] if LANG[0] == 'nl' else ACC[acc]
        midi = 12 * (octave + 1) + STEP[letter] + alter
        return midi, dia

    def post_events(self):
        # articulations, fingerings, dynamics, slurs, beams, ties
        tie = False
        while True:
            self.ws()
            c = self.peek()
            if c == '~':
                tie = True
                self.i += 1
            elif c in '()[]':
                self.i += 1
            elif c in '-^_':
                self.i += 1
                c2 = self.peek()
                if c2 == '\\':
                    w = self.word()
                    if w == 'markup':
                        self.skip_markup()
                elif c2 == '"':
                    self.skip_string()
                elif c2 == '#':
                    self.skip_scheme()
                else:
                    self.i += 1  # . > | - ^ _ digit etc.
                    while self.peek().isdigit():
                        self.i += 1
            elif c == '\\':
                j = self.i + 1
                w = re.match(r"[A-Za-z]+|[!<>()]", self.s[j:])
                name = w.group(0) if w else ''
                # dynamics / spanners attached to the note
                if name in ('(', ')', '<', '>', '!') or name in ('f', 'p', 'pp', 'ppp', 'mf', 'mp', 'ff', 'fff', 'sf', 'sfz', 'fp', 'fz', 'cresc', 'decresc', 'dim',
                                                                   'startTrillSpan', 'stopTrillSpan', 'startTextSpan', 'stopTextSpan', 'fermata', 'trill', 'prall', 'turn', 'mordent',
                                                                   'staccato', 'accent', 'tenuto', 'marcato', 'ped', 'pes', 'ddd', 'grc', 'prl', 'pru', 'tei', 'spp', 'dol', 'arpeggio', 'sustainOn', 'sustainOff'):
                    self.i = j + len(name)
                else:
                    break
            else:
                break
        return tie

    def add_note(self, midi, dur, chord_extra=False):
        if midi in self.tied:
            idx = self.tied.pop(midi)
            ev = self.events[idx]
            if abs(ev['s'] + ev['d'] - self.t) < 1e-6 or chord_extra:
                ev['d'] = self.t + dur - ev['s']
                return idx
        self.events.append({'p': midi, 's': self.t, 'd': dur if not self.grace else 0.0, 'h': self.hand, 'g': self.grace})
        return len(self.events) - 1

    def note_or_chord(self):
        c = self.peek()
        if c == '<' and self.s[self.i:self.i + 2] != '<<':
            self.i += 1
            pitches = []
            first_dia = None
            while True:
                self.ws()
                if self.peek() == '>':
                    self.i += 1
                    break
                m = NOTE_RE.match(self.s, self.i)
                if not m:
                    # fingering etc inside chord
                    if self.peek() in '-^_':
                        self.i += 1
                        if self.peek() == '\\':
                            w = self.word()
                            if w == 'markup':
                                self.skip_markup()
                        else:
                            self.i += 1
                            while self.peek().isdigit():
                                self.i += 1
                        continue
                    if self.peek() == '\\':
                        self.word()
                        continue
                    if self.peek() == '#':
                        self.skip_scheme()
                        continue
                    self.i += 1
                    continue
                self.i = m.end()
                midi, dia = self.abs_pitch(m.group(1), m.group(2) or '', m.group(3))
                if self.ref is not None:
                    self.ref = dia
                if first_dia is None:
                    first_dia = dia
                pitches.append(midi)
            d = self.read_duration()
            if d is not None:
                self.dur = d
            dur = self.dur * self.scale
            tie = self.post_events()
            if self.ref is not None and first_dia is not None:
                self.ref = first_dia
            newtied = {}
            for p in pitches:
                idx = self.add_note(p, dur)
                if tie:
                    newtied[p] = idx
            self.tied.update(newtied)
            if not self.grace:
                self.t += dur
            return True
        m = NOTE_RE.match(self.s, self.i)
        if m and (m.end() >= len(self.s) or not self.s[m.end()].isalpha()):
            self.i = m.end()
            d = None
            if m.group(5):
                self.i -= len(m.group(5))
                d = self.read_duration()
            else:
                save = self.i
                d = self.read_duration()
                if d is None:
                    self.i = save
            if d is not None:
                self.dur = d
            dur = self.dur * self.scale
            midi, dia = self.abs_pitch(m.group(1), m.group(2) or '', m.group(3))
            if self.ref is not None:
                self.ref = dia
            tie = self.post_events()
            idx = self.add_note(midi, dur)
            if tie:
                self.tied[midi] = idx
            if not self.grace:
                self.t += dur
            return True
        # rests / spacers
        m = re.match(r"([rsR])(\d+\.*)?", self.s[self.i:])
        if m and (self.i + m.end() >= len(self.s) or not self.s[self.i + m.end()].isalpha()):
            self.i += 1
            if m.group(2):
                d = self.read_duration()
                self.dur = d
            mult = 1
            if self.peek() == '*':
                self.i += 1
                mm = re.match(r"(\d+)(/(\d+))?", self.s[self.i:])
                self.i += mm.end()
                mult = int(mm.group(1)) / (int(mm.group(3)) if mm.group(3) else 1)
            self.post_events()
            if not self.grace:
                self.t += self.dur * self.scale * mult
            return True
        return False

    def music(self, until=None):
        while self.i < len(self.s):
            self.ws()
            c = self.peek()
            if not c:
                return
            if until and self.s.startswith(until, self.i):
                return
            if c == '{':
                self.i += 1
                self.music('}')
                self.i += 1
                continue
            if c == '}':
                return
            if self.s.startswith('<<', self.i):
                self.i += 2
                self.simultaneous()
                continue
            if c == '"':
                self.skip_string()
                continue
            if c == '#':
                self.skip_scheme()
                continue
            if c == '|':
                self.i += 1
                continue
            if c == '\\':
                if self.s.startswith('\\\\', self.i):
                    return
                w = self.word()
                self.command(w)
                continue
            before = self.i
            if self.note_or_chord():
                continue
            # unknown char / word: skip a token (always make progress)
            if c not in '()[]~':
                self.word_plain()
            if self.i == before:
                self.i += 1

    def simultaneous(self):
        start = self.t
        ref0 = self.ref
        ends = []
        refs = []
        while self.i < len(self.s):
            self.ws()
            if self.s.startswith('>>', self.i):
                self.i += 2
                break
            if self.s.startswith('\\\\', self.i):
                self.i += 2
                continue
            self.t = start
            self.ref = ref0
            if self.peek() == '{':
                self.i += 1
                self.music('}')
                self.i += 1
            elif self.peek() == '\\':
                w = self.word()
                self.command(w)
            else:
                b = self.i
                self.music('>>')
                if self.i == b:
                    self.i += 1
            ends.append(self.t)
            refs.append(self.ref)
        self.t = max(ends) if ends else start
        if refs:
            self.ref = refs[0]

    def command(self, w):
        if w in ('relative',):
            self.ws()
            m = NOTE_RE.match(self.s, self.i)
            self.i = m.end()
            octave = 3 + m.group(3).count("'") - m.group(3).count(',')
            self.ref = octave * 7 + DIA[m.group(1)]
            return
        if w in ('clef', 'key', 'time', 'tempo', 'bar', 'mark', 'set', 'unset', 'override', 'revert', 'once', 'tupletUp', 'tupletDown',
                 'include', 'version'):
            if w == 'once':
                return
            if w in ('override', 'set', 'revert', 'unset'):
                # skip property path and optional "= value"
                self.ws()
                self.word_plain()
                self.ws()
                if self.peek() == '#':
                    self.skip_scheme()
                    self.ws()
                if self.peek() == '=':
                    self.i += 1
                    self.ws()
                    if self.peek() == '#':
                        self.skip_scheme()
                    elif self.peek() == '\\':
                        self.word()
                        if self.peek() == ' ':
                            pass
                    else:
                        self.word_plain()
                return
            if w == 'key':
                self.ws(); self.word_plain(); self.ws(); self.word()
                return
            if w == 'tempo':
                self.ws(); self.word_plain()
                return
            if w in ('mark', 'bar'):
                self.ws()
                if self.peek() == '"':
                    self.skip_string()
                elif self.peek() == '\\':
                    ww = self.word()
                    if ww == 'markup':
                        self.skip_markup()
                return
            self.ws()
            if self.peek() == '"':
                self.skip_string()
            else:
                self.word_plain()
            return
        if w == 'partial':
            self.ws()
            d = self.read_duration()
            self.partial = d
            return
        if w == 'times':
            self.ws()
            m = re.match(r"(\d+)/(\d+)", self.s[self.i:])
            self.i += m.end()
            old = self.scale
            self.scale = old * int(m.group(1)) / int(m.group(2))
            self.ws()
            if self.peek() == '{':
                self.i += 1
                self.music('}')
                self.i += 1
            self.scale = old
            return
        if w in ('grace', 'acciaccatura', 'appoggiatura', 'afterGrace'):
            old = self.grace
            self.grace = True
            self.ws()
            if self.peek() == '{':
                self.i += 1
                self.music('}')
                self.i += 1
            else:
                self.note_or_chord()
            self.grace = old
            return
        if w == 'markup':
            self.skip_markup()
            return
        if w in ('tiny', 'small', 'normalsize', 'teeny'):
            return
        # a user-defined music variable: parse its body in place
        if w in VARS:
            saved_s, saved_i = self.s, self.i
            self.s, self.i = VARS[w], 0
            self.music()
            self.s, self.i = saved_s, saved_i
            return
        # anything else: a no-argument command (\stemUp, \cadenzaOn, user macros ...)
        return


def block_after(src, pos):
    j = src.index('{', pos)
    depth = 0
    k = j
    while True:
        if src[k] == '{':
            depth += 1
        elif src[k] == '}':
            depth -= 1
            if depth == 0:
                return j, k
        k += 1


def parse(path):
    global NOTE_RE
    raw = open(path, encoding='utf-8', errors='replace').read()
    src = strip_comments(raw)
    LANG[0] = 'en' if 'english.ly' in src else 'nl'
    NOTE_RE = NOTE_RE_EN if LANG[0] == 'en' else NOTE_RE_NL
    VARS.clear()
    for m in re.finditer(r"\n([A-Za-z]+)\s*=\s*\{", src):
        j, k = block_after(src, m.end() - 1)
        VARS[m.group(1)] = src[j:k + 1]
    names = None
    for up, lo in (('upper', 'lower'), ('upperStaff', 'lowerStaff'), ('rightNotes', 'leftNotes')):
        if re.search(r"\n\s*" + up + r"\s*=\s*\\relative", src):
            names = (up, lo)
            break
    out = []
    for name, hand in ((names[0], 0), (names[1], 1)):
        m = re.search(r"\n\s*" + name + r"\s*=\s*\\relative", src)
        j, k = block_after(src, m.end())
        body = src[m.end() - len('\\relative'):k + 1]
        p = Parser(body, hand)
        p.music()
        for e in p.events:
            e['s'] = round(e['s'] - (p.partial or 0), 6)
        out += p.events
    out.sort(key=lambda e: (e['s'], e['h'], e['p']))
    return out


if __name__ == '__main__':
    ev = parse(sys.argv[1])
    json.dump(ev, open(sys.argv[2], 'w'))
    for h in (0, 1):
        hs = [e for e in ev if e['h'] == h]
        print('hand', h, 'notes', len(hs), 'end', max(e['s'] + e['d'] for e in hs))
    names = ['C', 'C#', 'D', 'Eb', 'E', 'F', 'F#', 'G', 'Ab', 'A', 'Bb', 'B']
    print('first RH:', ' '.join(names[e['p'] % 12] + str(e['p'] // 12 - 1) for e in ev if e['h'] == 0)[:200])
    print('first LH:', ' '.join(names[e['p'] % 12] + str(e['p'] // 12 - 1) for e in ev if e['h'] == 1)[:200])
