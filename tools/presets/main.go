// Command presets builds the built-in song collection (presets/songs.json.gz)
// from the MIDI files in presets/midi.
//
// Two-hand piano files are used as they are. Band and orchestra files are
// reduced to a playable piano part: the melody instruments become the right
// hand (top note only) and the bass instruments the left hand (bottom note
// only), each moved by octaves into a comfortable range.
//
//	go run ./tools/presets
package main

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"

	"github.com/codypowpow-afk/piano-lessons/score"
)

// preset describes one built-in song. With no RH/LH tracks the file is a
// piano score and is read as it is.
type preset struct {
	file, title string
	rh, lh      []int // tracks for the reduction (melody, bass)
}

type category struct {
	name  string
	items []preset
}

var collection = []category{
	{"Classical", []preset{
		{file: "fur-elise", title: "Beethoven — Für Elise"},
		{file: "sonate-pathetique-adagio", title: "Beethoven — Pathétique Sonata, Adagio"},
		{file: "menuet-en-sol", title: "Petzold (Bach notebook) — Minuet in G major"},
		{file: "menuet-sol-mineur", title: "Petzold (Bach notebook) — Minuet in G minor"},
		{file: "bach-musette", title: "Bach — Musette in D major"},
		{file: "bach-invention-1", title: "Bach — Invention No. 1 in C major"},
		{file: "prelude-bwv-847", title: "Bach — Prelude in C minor, BWV 847"},
		{file: "brahms-valse-15", title: "Brahms — Waltz in A-flat, Op. 39 No. 15"},
		{file: "burgmuller-arabesque", title: "Burgmüller — Arabesque, Op. 100 No. 2"},
		{file: "burgmuller-ballade", title: "Burgmüller — Ballade, Op. 100 No. 15"},
		{file: "nocturne-op9-no2", title: "Chopin — Nocturne Op. 9 No. 2 (MIDI)"},
		{file: "nocturne-op72-no1", title: "Chopin — Nocturne Op. 72 No. 1 in E minor"},
		{file: "prelude-mi-mineur", title: "Chopin — Prelude Op. 28 No. 4 in E minor"},
		{file: "chopin-prelude-6", title: "Chopin — Prelude Op. 28 No. 6 in B minor"},
		{file: "chopin-prelude-7", title: "Chopin — Prelude Op. 28 No. 7 in A major"},
		{file: "prelude-goutte-eau", title: "Chopin — Prelude Op. 28 No. 15 \"Raindrop\""},
		{file: "chopin-prelude-20", title: "Chopin — Prelude Op. 28 No. 20 in C minor"},
		{file: "clair-de-lune", title: "Debussy — Clair de lune"},
		{file: "arabesque-1-debussy", title: "Debussy — Arabesque No. 1"},
		{file: "arabesque-2-debussy", title: "Debussy — Arabesque No. 2"},
		{file: "antre-du-roi-de-la-montagne", title: "Grieg — In the Hall of the Mountain King"},
		{file: "consolation-3", title: "Liszt — Consolation No. 3"},
		{file: "sonata-facile", title: "Mozart — Sonata K. 545 \"Facile\""},
		{file: "sonate-k331-theme", title: "Mozart — Sonata K. 331, Theme"},
		{file: "mozart-fantaisie-re-mineur", title: "Mozart — Fantasia in D minor, K. 397"},
		{file: "gymnopedie-1", title: "Satie — Gymnopédie No. 1"},
		{file: "gymnopedie-2", title: "Satie — Gymnopédie No. 2"},
		{file: "gymnopedie-3", title: "Satie — Gymnopédie No. 3"},
		{file: "gnossienne-1", title: "Satie — Gnossienne No. 1"},
		{file: "gnossienne-2", title: "Satie — Gnossienne No. 2"},
		{file: "kinderszenen-1", title: "Schumann — Of Foreign Lands and Peoples"},
		{file: "traumerei", title: "Schumann — Träumerei"},
		{file: "beau-danube-bleu", title: "Strauss — The Blue Danube"},
		{file: "marche-soldats-bois", title: "Tchaikovsky — March of the Wooden Soldiers"},
		{file: "vieille-chanson-francaise", title: "Tchaikovsky — Old French Song"},
	}},
	{"Ragtime", []preset{
		{file: "the-entertainer-easy", title: "Joplin — The Entertainer (easy)"},
		{file: "the-entertainer", title: "Joplin — The Entertainer"},
		{file: "peacherine-rag", title: "Joplin — Peacherine Rag"},
		{file: "pineapple-rag", title: "Joplin — Pineapple Rag"},
	}},
	{"Movie & game themes", []preset{
		{file: "addams-family", title: "The Addams Family", rh: []int{1}, lh: []int{5, 4}},
		{file: "ghostbusters", title: "Ghostbusters", rh: []int{1, 2}, lh: []int{3, 2}},
		{file: "james-bond", title: "James Bond Theme", rh: []int{4, 13, 12, 3}, lh: []int{1}},
		{file: "jurassic-park", title: "Jurassic Park", rh: []int{1, 5, 3}, lh: []int{9, 10, 2}},
		{file: "star-wars-duel-of-the-fates", title: "Star Wars — Duel of the Fates", rh: []int{1, 9, 7, 4}, lh: []int{3}},
		{file: "doki-doki-literature-club-main-theme", title: "Doki Doki Literature Club — Main Theme", rh: []int{3, 5, 12, 13, 15}, lh: []int{10}},
	}},
	{"Pop & rock", []preset{
		{file: "cant-help-falling-in-love", title: "Can't Help Falling in Love", rh: []int{3, 2}, lh: []int{2}},
		{file: "queen-another-one-bites-the-dust", title: "Queen — Another One Bites the Dust", rh: []int{1, 6}, lh: []int{3}},
		{file: "queen-bohemian-rhapsody", title: "Queen — Bohemian Rhapsody", rh: []int{6, 1, 2, 9}, lh: []int{4, 3}},
	}},
}

type note struct{ tick, dur, pitch, vel int }

type smf struct {
	div    int
	meta   [][]byte // tempo and time-signature events: delta-free (tick, bytes)
	mticks []int
	tracks [][]note
	chans  [][]int
}

func parse(data []byte) (*smf, error) {
	if len(data) < 14 || string(data[:4]) != "MThd" {
		return nil, fmt.Errorf("not a MIDI file")
	}
	ntrk := int(binary.BigEndian.Uint16(data[10:12]))
	m := &smf{div: int(binary.BigEndian.Uint16(data[12:14]))}
	pos := 8 + int(binary.BigEndian.Uint32(data[4:8]))
	for t := 0; t < ntrk && pos+8 <= len(data); t++ {
		l := int(binary.BigEndian.Uint32(data[pos+4 : pos+8]))
		trk := data[pos+8 : min(len(data), pos+8+l)]
		pos += 8 + l
		var notes []note
		var chans []int
		open := map[[2]int]int{}
		i, tick := 0, 0
		var status byte
		rv := func() int {
			v := 0
			for i < len(trk) {
				b := trk[i]
				i++
				v = v<<7 | int(b&0x7f)
				if b&0x80 == 0 {
					break
				}
			}
			return v
		}
		for i < len(trk) {
			tick += rv()
			if i >= len(trk) {
				break
			}
			if trk[i]&0x80 != 0 {
				status = trk[i]
				i++
			}
			switch {
			case status == 0xFF:
				typ := trk[i]
				i++
				n := rv()
				d := trk[i:min(len(trk), i+n)]
				i += n
				if typ == 0x51 || typ == 0x58 {
					m.meta = append(m.meta, append([]byte{0xFF, typ, byte(len(d))}, d...))
					m.mticks = append(m.mticks, tick)
				}
			case status == 0xF0 || status == 0xF7:
				i += rv()
			default:
				h := status & 0xF0
				n := 2
				if h == 0xC0 || h == 0xD0 {
					n = 1
				}
				if i+n > len(trk) {
					i = len(trk)
					break
				}
				d1, d2 := int(trk[i]), 0
				if n == 2 {
					d2 = int(trk[i+1])
				}
				i += n
				ch := int(status & 15)
				k := [2]int{d1, ch}
				if h == 0x90 && d2 > 0 {
					if ch == 9 {
						continue // drums
					}
					if j, ok := open[k]; ok { // re-struck before release
						notes[j].dur = tick - notes[j].tick
					}
					open[k] = len(notes)
					notes = append(notes, note{tick, 0, d1, d2})
					chans = append(chans, ch)
				} else if h == 0x80 || h == 0x90 {
					if j, ok := open[k]; ok {
						notes[j].dur = tick - notes[j].tick
						delete(open, k)
					}
				}
			}
		}
		m.tracks = append(m.tracks, notes)
		m.chans = append(m.chans, chans)
	}
	return m, nil
}

// line reduces the given tracks to a single voice. Tracks are in priority
// order: in each bar the first track that plays there is used, so the line
// follows the tune from instrument to instrument instead of mixing them.
// Per onset the highest (top) or lowest note is kept and each note is cut off
// where the next starts. Each track is first moved by whole octaves so its
// median pitch lands near centre.
func line(m *smf, tracks []int, top bool, centre int) []note {
	bar := m.div * 4
	owner := map[int]int{} // bar -> priority index of the track used
	var parts [][]note
	for pi, t := range tracks {
		var part []note
		if t < len(m.tracks) && len(m.tracks[t]) > 0 {
			part = octaveTo(m.tracks[t], centre, top)
		}
		parts = append(parts, part)
		for _, n := range part {
			b := n.tick / bar
			if _, ok := owner[b]; !ok {
				owner[b] = pi
			}
		}
	}
	var all []note
	for pi, part := range parts {
		for _, n := range part {
			if owner[n.tick/bar] == pi {
				all = append(all, n)
			}
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].tick < all[j].tick })
	win := m.div / 12 // notes this close together count as one onset
	var out []note
	for i := 0; i < len(all); {
		j, best := i, all[i]
		for j < len(all) && all[j].tick-all[i].tick <= win {
			if (top && all[j].pitch > best.pitch) || (!top && all[j].pitch < best.pitch) {
				best = all[j]
			}
			j++
		}
		best.tick = all[i].tick
		out = append(out, best)
		i = j
	}
	for i := range out {
		if out[i].dur < m.div/16 {
			out[i].dur = m.div / 16
		}
		if i+1 < len(out) && out[i].tick+out[i].dur > out[i+1].tick {
			out[i].dur = max(1, out[i+1].tick-out[i].tick)
		}
	}
	return out
}

// octaveTo moves a part by whole octaves so the median pitch of the notes
// the line will use (the top or bottom note of each onset) is near centre.
// Notes still off the piano are folded back onto it.
func octaveTo(notes []note, centre int, top bool) []note {
	edge := map[int]int{}
	for _, n := range notes {
		e, ok := edge[n.tick]
		if !ok || (top && n.pitch > e) || (!top && n.pitch < e) {
			edge[n.tick] = n.pitch
		}
	}
	var ps []int
	for _, p := range edge {
		ps = append(ps, p)
	}
	sort.Ints(ps)
	shift := int(math.Round(float64(centre-ps[len(ps)/2])/12)) * 12
	out := make([]note, len(notes))
	for i, n := range notes {
		n.pitch += shift
		for n.pitch < 28 {
			n.pitch += 12
		}
		for n.pitch > 103 {
			n.pitch -= 12
		}
		out[i] = n
	}
	return out
}

func vlq(v int) []byte {
	b := []byte{byte(v & 0x7f)}
	for v >>= 7; v > 0; v >>= 7 {
		b = append([]byte{byte(v&0x7f | 0x80)}, b...)
	}
	return b
}

// write builds a three-track SMF: conductor, right hand, left hand.
func write(m *smf, hands ...[]note) []byte {
	type ev struct {
		tick int
		b    []byte
	}
	chunk := func(evs []ev) []byte {
		sort.SliceStable(evs, func(i, j int) bool { return evs[i].tick < evs[j].tick })
		var body []byte
		last := 0
		for _, e := range evs {
			body = append(body, vlq(e.tick-last)...)
			body = append(body, e.b...)
			last = e.tick
		}
		body = append(body, 0, 0xFF, 0x2F, 0)
		h := []byte("MTrk")
		h = binary.BigEndian.AppendUint32(h, uint32(len(body)))
		return append(h, body...)
	}
	out := []byte("MThd")
	out = binary.BigEndian.AppendUint32(out, 6)
	out = binary.BigEndian.AppendUint16(out, 1)
	out = binary.BigEndian.AppendUint16(out, uint16(1+len(hands)))
	out = binary.BigEndian.AppendUint16(out, uint16(m.div))
	var cond []ev
	for i, b := range m.meta {
		cond = append(cond, ev{m.mticks[i], b})
	}
	out = append(out, chunk(cond)...)
	for h, notes := range hands {
		var evs []ev
		for _, n := range notes {
			p := byte(min(127, max(0, n.pitch)))
			// note-offs sort before note-ons at the same tick
			evs = append(evs, ev{n.tick + n.dur, []byte{0x80 | byte(h), p, 0}})
		}
		for _, n := range notes {
			p := byte(min(127, max(0, n.pitch)))
			evs = append(evs, ev{n.tick, []byte{0x90 | byte(h), p, byte(max(1, n.vel))}})
		}
		out = append(out, chunk(evs)...)
	}
	return out
}

// performed reports whether the file was recorded from a performance rather
// than written out: hardly any notes fall on a sixteenth-note grid.
func performed(m *smf) bool {
	on, all := 0, 0
	for _, t := range m.tracks {
		for _, n := range t {
			g := float64(n.tick) * 4 / float64(m.div)
			if math.Abs(g-math.Round(g)) < 0.08 {
				on++
			}
			all++
		}
	}
	return float64(on) < 0.5*float64(all)
}

// snapChords lines up the notes of a played chord (which never start at
// exactly the same moment in a performance) so the lesson treats them as one
// chord. Notes within 45 ms of the first note of a group are moved to it.
func snapChords(s *score.Song) {
	eps := 0.045 * s.Tempo / 60
	s.Sort()
	first := math.Inf(-1)
	for i := range s.Notes {
		n := &s.Notes[i]
		if n.Start-first > eps {
			first = n.Start
			continue
		}
		n.Dur += n.Start - first
		n.Start = first
	}
	s.Sort()
	for i := range s.Notes {
		s.Notes[i].Measure = s.MeasureAt(s.Notes[i].Start)
	}
}

type outCat struct {
	Category string        `json:"category"`
	Songs    []*score.Song `json:"songs"`
}

func main() {
	dir := "presets/midi"
	var cats []outCat
	total := 0
	for _, c := range collection {
		oc := outCat{Category: c.name}
		for _, p := range c.items {
			data, err := os.ReadFile(filepath.Join(dir, p.file+".mid"))
			if err != nil {
				log.Fatal(err)
			}
			src := "MIDI"
			m, err := parse(data)
			if err != nil {
				log.Fatalf("%s: %v", p.file, err)
			}
			perf := performed(m)
			if p.rh != nil {
				rh, lh := line(m, p.rh, true, 72), line(m, p.lh, false, 45)
				data = write(m, rh, lh)
				src = "MIDI, arranged for piano: melody in the right hand, bass line in the left"
			}
			s, err := score.ReadMIDI(data, p.title)
			if err != nil {
				log.Fatalf("%s: %v", p.file, err)
			}
			s.Title, s.Source = p.title, src
			if perf {
				snapChords(s)
				s.Source = "MIDI (performance recording: bar lines are every 4 seconds, not the score's bars)"
			}
			var r, l int
			for _, n := range s.Notes {
				if n.Hand == score.Right {
					r++
				} else {
					l++
				}
			}
			lo, hi := s.PitchRange()
			fmt.Printf("%-48s %5d notes (R %4d L %4d) %3.0f bpm %3d bars %s-%s perf=%v\n",
				p.title, len(s.Notes), r, l, s.Tempo, len(s.Measures), score.NoteName(lo), score.NoteName(hi), perf)
			total += len(s.Notes)
			oc.Songs = append(oc.Songs, s)
		}
		cats = append(cats, oc)
	}
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err := json.NewEncoder(zw).Encode(cats); err != nil {
		log.Fatal(err)
	}
	zw.Close()
	if err := os.WriteFile("presets/songs.json.gz", buf.Bytes(), 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%d notes, presets/songs.json.gz %d KB\n", total, buf.Len()/1024)
}
