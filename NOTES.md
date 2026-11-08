# Piano Lessons — Project Notes & Handoff

Its own repo since 2026-11-07 (split out of Piano Player, whose `synth`/`assets` it imports). It turns sheet music into falling notes over the 88-key
piano and waits for you to play each note. Written at the end of the build session on 2026-11-05.

- **Build:** `GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w -H=windowsgui" -o dist/PianoLessons.exe .`
- **Size:** ~48 MB exe (same samples as Piano Player plus a ~0.5 MB sample song file)
- **New code:** ~6,300 lines of Go (incl. tests and tools): OMR reader, song model and importers, lesson engine, UI
- **Engine:** `github.com/codypowpow-afk/piano-player` (`synth`, `assets`) as a Go dependency

---

## 1. What it does

1. **Import**: scanned PDF, PNG/JPG page images, MIDI, MusicXML (`.musicxml`, `.xml`, `.mxl`), or
   the app's own JSON. Drag and drop, **Open file…** (Windows dialog), or a command-line argument
   (so "Open with…" and dropping a file onto the exe both work).
2. **Read the music** (PDF/images only): built-in optical music recognition (OMR), see §4.
   A PDF holding several pieces is split into pieces (Chopin Op. 9 → No. 1, 2, 3).
3. **Learn**: notes fall onto the keys. Right hand teal, left hand amber; the app's own notes
   are dimmed. The current step is outlined and its keys are lit on the keyboard.
   - **Wait mode**: the song stops at each step until every key of that step has been pressed
     (a fresh press is needed, so repeated notes count).
   - **Hands**: each hand is **You** / **Auto** / **Off** (F1 right, F5 left). Off hands are
     silent and drawn as outlines, so melody and chords can be learned separately.
   - **Small keyboards**: MIDI octave shift −4…+4 (Shift+←/→; a key released after a shift
     still stops the note it started) and **Any octave** (F6), which matches by note name.
   - **Listen**: the app plays everything.
   - **Loop** 1/2/4/8 bars, **tempo** 25–150 %, **Skip** (Tab) a note the reader got wrong.
   - Scoring: right notes, wrong notes (only counted near a step), skipped, accuracy.
4. **Library**: every import is saved as JSON in `%AppData%\PianoLessons\library\`.
   **Export MIDI** writes `Documents\Piano Lessons\<title>.mid` (right hand on track 1,
   left on track 2), so a misread score can be fixed in MuseScore and re-imported.

---

## 2. File map

```
main.go                     UI (Ebiten): layout, buttons, waterfall, keyboard, overlays, input
library.go                  Import dispatch by file type, library folder, embedded sample
dialog_windows.go           Native Open dialog (comdlg32 GetOpenFileNameW, no cgo)
dialog_other.go             No dialog elsewhere (drag and drop still works)
sample/chopin_op9.json           The three Op. 9 nocturnes as read from the scan
rsrc_windows_amd64.syso          Icon + version info ("Piano Lessons")
internal/lesson/            Lesson state machine (wait mode, steps, auto-play, loop, scoring) + tests
internal/midiin/            The winmm MIDI input (copied from Piano Player)
internal/appicon/           App icon (copied from Piano Player)
score/score.go              Song / Note / Step model, JSON, note names
score/midi.go               MIDI reader (tracks/channels -> hands, tempo, time signature) + writer
score/musicxml.go           MusicXML reader (partwise, staves, chords, backup/forward, ties, .mxl)
omr/pdf.go                  Tiny PDF reader: object streams, page tree, CCITT G3/G4, JPEG, Flate
omr/imgops.go               Downscale, run stats, integral image, labelling, hole filling
omr/staff.go                Staff-line detection (strip-wise, warp tolerant), line removal
omr/recognize.go            Clefs, key signatures, noteheads, stems, barlines, accidentals
omr/ottava.go               "8 . . . . loco" detection
omr/ensemble.go             Thorough mode: several passes per page + voting
omr/build.go                Pages -> systems -> pieces -> measures -> timed notes
tools/sheet2song/           CLI: PDF -> songs JSON, prints the first bars of each piece
tools/omrdebug/             CLI: draws everything the reader found on one page into a PNG
tools/winres/               Regenerates rsrc_windows_amd64.syso
tools/eval/                 Accuracy measurement: LilyPond reference + alignment scorer
```

---

## 3. Song model (`score`)

```go
type Note struct { Pitch int; Start, Dur float64; Hand, Measure int; Vel float64; Page int; X, Y float32 }
type Song struct { Title string; Tempo, BeatsPer float64; KeyFifths int; Measures []float64; Notes []Note }
song.Steps(handMask, eps)   // notes grouped by onset = what the learner must press together
```
Times are in beats (quarter notes). For OMR songs every bar is 4 "beats" (the time signature
is not read), so the tempo is "bars feel"; MIDI/MusicXML keep their real meter and tempo.

---

## 4. The sheet-music reader (`omr`)

Classical, rule-based OMR. Each page is scaled so a staff space is 20 px, then:

1. **Staves**: the page is cut into vertical strips; in each strip, rows with a long horizontal
   ink run are line candidates. Groups of five evenly spaced lines are scored by line strength,
   linked across strips into chains, broken chains are merged, and the lines are tracked
   outward to the real staff ends. Lines are stored per strip, so warped scans are fine.
   Staves are paired into piano systems.
2. **Staff-line removal**: line pixels are erased where the vertical run is no thicker than a line.
3. **Noteheads**: two passes on the cleaned page: hollow heads (holes filled first) and filled
   heads. Each uses directional openings (keep horizontal runs ≥ 0.32 sp, then vertical runs
   ≥ 0.38 sp) that remove stems, ledger lines and thin curves. Blobs are split into chord heads
   using the row-width profile. Small (cue/cadenza) heads are accepted too.
4. **Stems**: a thin vertical line touching the head, traced with gap and 1-px wobble tolerance.
   A short stem is accepted when it ends in a beam. A "head" whose stem leaves on the wrong
   side is a flag unless a chord partner explains it. Blobs that run on sideways are beams.
   Stem direction decides the voice, the onset column, and which staff a note in the gap
   between the staves belongs to.
5. **Barlines**: columns covering the whole system top to bottom (strong), or both staves with
   no notehead next to them.
6. **Clef**: glyph pieces at the staff start (tall = treble, upper half = bass; otherwise
   treble on top, bass below). Mid-staff clef changes are looked for too.
7. **Key signature**: vertical strokes after the clef. One stroke per flat (longest run per
   column), close pairs per sharp (all runs, linked across columns). Each accidental must sit
   at the next position of the standard order. Per system, the best staff reading wins, and
   readings are smoothed across the piece (they fail by stopping early, never by inventing).
8. **Accidentals**: component classification plus a local stroke test just left of each head
   (natural = two offset strokes, sharp = two equal strokes, flat = one stroke + bowl). They
   apply for the rest of the bar at that staff position.
9. **8va**: an "8" followed by ≥ 6 evenly spaced dots above a staff.
10. **Timing**: within a bar, onset columns are clustered across both staves, and time is
    proportional to x position. Pickup bars are shortened, and very dense bars (cadenzas) are
    lengthened. A note lasts until the next onset in the same staff and stem voice.
11. **Pieces**: an indented first system starts a new piece.

### Thorough mode: several passes and a vote (since 1.2)

By default each page is read **four times**, at different analysis resolutions
(staff space 18, 20, 22.5 and 25.5 px). Every pass is a complete, independent read.
The passes are matched notehead by notehead:
- a notehead from the main (20 px) pass is kept if at least half the passes saw it;
- a notehead the main pass missed is added if all the other passes saw it;
- the staff position is a majority vote; an accidental seen by two passes wins.

This cancels most one-off mistakes from a single resolution. It costs about 3× the
time (a 12-page score: ~10 s → ~30 s). `omr.Quick()` / `sheet2song -quick` give the
old one-pass read.

Other accuracy work in 1.2:
- **Accidentals at full resolution**: the area left of every notehead is re-read on
  the original scan, where sharp/flat/natural strokes are still crisp.
- **Ties**: a thin arc between two same-pitch noteheads holds the first note instead
  of asking for it twice.
- **Ledger rule**: a note two or more steps outside the staff must sit on a ledger
  line; this removes "Ped." marks, fingering and dynamics read as notes.
- **Stems must touch their notehead**: a chord note no longer borrows the next chord's
  stem (which mistimed it).
- **Chord stacks** place their outer notes from the blob's edges; a notehead fused
  with its accidental is split by where the ink is, not down the middle.

### How good is it (Chopin Op. 9 scan, measured)
Measured with `tools/eval` against the Mutopia editions (an independent typesetting
of the same music), counting matching notes per hand:

| | 1.1 | 1.2 (thorough) |
|---|---|---|
| No. 2, bars 1–20 (reference reliable) | 83.0 % | 86.7 % |
| No. 2, whole piece | 75.5 % | 79.1 % |

The No. 2 numbers after bar 20 are held down partly by the reference converter and
the cadenza (small notes under 8va). No. 1's Mutopia edition differs from this scan
(different voicing in places), so it is only a rough check (35.6 % → 37.2 %).

What still goes wrong most: missed accidentals (now ~25 in No. 2's first 20 bars),
notes missed or added in dense ornaments and chords, the small-note cadenza, and
rhythm (timing within a bar is still by spacing).
- Staves 144/144, systems 72/72, pieces 3/3, each piece's key right (5♭, 3♭, 5♯).
- Bars: No. 2 reads as 35 bars = pickup + 34, matching the score.
- Vector PDFs (exported from notation software) have no page images: the app says so and
  suggests MIDI/MusicXML. JBIG2 and JPEG 2000 images are not decoded yet (export pages as PNG).

### Tuning workflow
`go run ./tools/omrdebug -o page.png score.pdf 5` draws every detection on page 5.
`go run ./tools/sheet2song -show 8 score.pdf` prints the first bars of each piece as note names.

---

## 5. Lesson engine (`internal/lesson`)

- `Advance(dt)` moves the position at `Tempo`. In wait mode it clamps at the next learner step.
- `Press(p)` records a key press with a sequence number. A step is done when every pitch was
  pressed after the previous step finished. Presses up to 0.6 beat early count.
- Auto-play plays every note that isn't the learner's (all of them in Listen mode) on time,
  releasing each one at its end.
- Fully unit tested without a window: `go test ./internal/lesson`.

---

## 6. Testing done in this session

- Unit tests: lesson engine (wait, chords, repeated notes, skip, loop, listen), MIDI round trip,
  MusicXML (chords, staves, backup, ties), OMR on a page image, key table.
- The UI was compiled to **WebAssembly** and run in headless Chrome. Screenshots confirmed
  layout, the waiting prompt, a correct key press advancing the song, the library overlay,
  hands and listen buttons, and playback. The real Windows window and a real MIDI keyboard
  were **not** tested (same as Piano Player); the MIDI code is the proven Piano Player backend.

## 7. Ideas / next steps
- Show the scanned page above the waterfall with the current notes boxed (notes already
  carry page/x/y).
- Tie detection; time-signature digits; triplet brackets.
- A small note editor (fix a misread pitch in place) instead of Export → edit → re-import.
- Optional Audiveris integration for users who have it installed.
- Remember settings (tempo, hands, last song) between runs.

## 8. Credits
Same as Piano Player (Salamander Grand Piano CC BY 3.0, VCSL CC0, Ebitengine, oggvorbis, winres,
Go fonts). The Chopin sample is read from a public-domain engraving.
