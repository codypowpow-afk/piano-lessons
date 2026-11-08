# Piano Lessons

Learn piano from sheet music. Give it a scanned PDF, page images, MIDI or MusicXML; the notes
fall onto an 88-key piano and **Wait** mode stops at every note or chord until you play it.

Download **PianoLessons.exe** from [Releases](https://github.com/codypowpow-afk/piano-lessons/releases) (or `download/` here). Windows x64, no installer; SmartScreen may warn because it isn't code-signed: **More info → Run anyway**.

It is built on the sound engine from [Piano Player](https://github.com/codypowpow-afk/piano-player). Give it sheet music
and it turns the notes into **falling blocks that land on the keys you need to play**. In
**Wait** mode it stops at every note or chord until you press the right keys, then moves on.

- **Open:** drag a file onto the window, use **Open file…** (`Ctrl+O`), or drop the file onto the exe.
  - **Scanned PDF** or **page images (PNG/JPG)**: read by the built-in sheet-music reader (no
    other software needed). Each page is read four times at different resolutions and the
    results are voted on, for accuracy; small cue notes (cadenzas, grace runs) are found on the
    full-resolution scan. A 12-page score takes about 30–40 s.
  - **MIDI** and **MusicXML (.musicxml / .mxl)**: exact, use these when you have them.
- Comes with **Chopin's Nocturnes Op. 9** (all three), read from a scanned PDF.
- Right hand is teal, left hand amber. Each hand has its own switch: **You** (you play it),
  **Auto** (the app plays it) or **Off** (silent), so you can learn the melody and the chords
  separately. Loop 1–8 bars, slow the tempo down, or press **Listen** to hear the piece.
- **Small MIDI keyboard?** Shift the keyboard by octaves (`Shift+←/→` or the − / + buttons), or
  turn on **Any octave** so the right note in any octave counts.
- Imported pieces are kept in the **Library** (`F10`). **Export MIDI** saves what was read, so
  you can fix it in any notation program and open the corrected file again.

| Key | Action |
|-----|--------|
| `Space` | play / pause |
| `Home` | restart |
| `Tab` | skip the current note (for a note the reader got wrong) |
| `F1` / `F5` | right hand / left hand: You → Auto → Off |
| `F2` / `F3` / `F4` / `F6` | wait mode · listen · loop · any octave |
| `Shift+←` `Shift+→` | MIDI keyboard octave down / up |
| `↑` `↓` | tempo |
| `PgUp` `PgDn` | previous / next bar · mouse wheel scrolls through the song while paused |
| `F7` `F8` · `F9` · `F10` | sound · note names · library |
| `Z`…`/`, `Q`…`]`, `←` `→`, `Shift` | computer-keyboard piano, its octave, sustain |

The sheet-music reader works on clean engraved scans. It reliably finds staves, clefs, key
signatures, barlines, noteheads and chords. Accidentals inside a bar and complex rhythms are
harder, so some notes may be wrong: press `Tab` to skip one, or use MIDI/MusicXML for perfect
input. See [`NOTES.md`](NOTES.md) for how it works.

## Build it yourself

Go 1.24, no cgo needed for Windows:

```bash
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w -H=windowsgui" -o dist/PianoLessons.exe .
go test ./internal/lesson ./score ./omr
go run ./tools/sheet2song -show 8 score.pdf      # read a PDF from the command line
go run ./tools/omrdebug -o page.png score.pdf 5  # draw what the reader found on page 5
```

The piano sound (sampler engine and samples) comes from the
`github.com/codypowpow-afk/piano-player` module (`synth` and `assets` packages).

## Credits

- Piano sounds: **Salamander Grand Piano V3** by Alexander Holm (CC BY 3.0; 4 of 16 layers,
  trimmed, level-matched, re-encoded to Ogg Vorbis) and the **Versilian Community Sample
  Library** (CC0), via Piano Player.
- Ebitengine (Apache 2.0), jfreymuth/oggvorbis (MIT), tc-hib/winres (0BSD), Go fonts (BSD).
- The Chopin sample was read from a public-domain engraving.
