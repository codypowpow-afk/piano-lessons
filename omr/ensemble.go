package omr

import (
	"math"
	"sort"
)

// passScales are the analysis resolutions (staff space in px) used for each
// page. With more than one, every page is read several times and the results
// are combined by voting, which cancels most single-pass mistakes.
var passScales = []float64{20}

// voting thresholds (0 = defaults); see analyzeEnsemble
var voteKeep, voteAdd = 0, 0

// SetVoting overrides how many passes must agree to keep a base-pass notehead
// and to add a notehead the base pass missed.
func SetVoting(keep, add int) { voteKeep, voteAdd = keep, add }

// SetPasses chooses the analysis resolutions. A single value is a quick
// one-pass read; several values give the slower, more accurate consensus read.
func SetPasses(scales ...float64) {
	if len(scales) == 0 {
		scales = []float64{20}
	}
	passScales = scales
}

// Thorough reads each page at four resolutions and votes.
func Thorough() { SetPasses(18, 20, 22.5, 25.5); SetVoting(1, 2) }

// Quick reads each page once.
func Quick() { SetPasses(20); SetVoting(0, 0) }

type candidate struct {
	h    *Head
	runs map[int]bool
	n    int
}

func analyzeEnsemble(full *Bitmap) *PageResult {
	var runs []*PageResult
	baseIdx := 0
	for i, sc := range passScales {
		runs = append(runs, analyzeAt(full, sc))
		if math.Abs(sc-20) < math.Abs(passScales[baseIdx]-20) {
			baseIdx = i
		}
	}
	base := runs[baseIdx]
	if len(base.Staves) == 0 {
		return base
	}
	type tally struct {
		votes int
		pos   map[int]int
		acc   map[int]int
	}
	tl := map[*Head]*tally{}
	for _, h := range base.Heads {
		tl[h] = &tally{votes: 1, pos: map[int]int{h.Pos: 1}, acc: map[int]int{h.Acc: 1}}
	}
	sp := base.Space
	var cands []*candidate
	used := 1
	for ri, r := range runs {
		if ri == baseIdx || len(r.Staves) != len(base.Staves) {
			continue
		}
		used++
		k := r.fscale / base.fscale // run px -> base px
		for _, hr := range r.Heads {
			x, y := hr.X*k, hr.Y*k
			var best *Head
			bd := math.Inf(1)
			for _, hb := range base.Heads {
				if hb.Staff != hr.Staff {
					continue
				}
				dx, dy := math.Abs(hb.X-x), math.Abs(hb.Y-y)
				if dx < sp*0.55 && dy < sp*0.4 {
					if d := dx + 2*dy; d < bd {
						best, bd = hb, d
					}
				}
			}
			if best != nil {
				t := tl[best]
				t.votes++
				t.pos[hr.Pos]++
				t.acc[hr.Acc]++
				continue
			}
			// not in the base pass: collect, grouping matches from other runs
			var c *candidate
			for _, cc := range cands {
				if cc.h.Staff == hr.Staff && math.Abs(cc.h.X-x) < sp*0.55 && math.Abs(cc.h.Y-y) < sp*0.4 && !cc.runs[ri] {
					c = cc
					break
				}
			}
			if c == nil {
				nh := *hr
				nh.X, nh.Y, nh.W = x, y, hr.W*k
				nh.StemX, nh.OnsetX = hr.StemX*k, hr.OnsetX*k
				nh.tiedFrom, nh.TiedIn = nil, false
				c = &candidate{h: &nh, runs: map[int]bool{}}
				cands = append(cands, c)
			}
			c.runs[ri] = true
			c.n++
		}
	}
	if used < 3 {
		base.findTies()
		return base
	}
	need := (used + 1) / 2 // keep base heads seen by at least half the passes
	if voteKeep > 0 {
		need = voteKeep
	}
	var out []*Head
	for _, h := range base.Heads {
		t := tl[h]
		if t.votes < need {
			continue
		}
		h.Pos = argmaxInt(t.pos, h.Pos)
		// accidentals are mostly missed, rarely invented: any kind seen by
		// two passes wins; otherwise keep what the base pass found
		bestK, bestN := 0, 0
		for kd, n := range t.acc {
			if kd != 0 && n > bestN {
				bestK, bestN = kd, n
			}
		}
		if bestN >= 2 {
			h.Acc = bestK
		}
		out = append(out, h)
	}
	for _, c := range cands {
		addNeed := used - 1
		if voteAdd > 0 {
			addNeed = voteAdd
		}
		if c.n >= addNeed && c.n >= 1 { // seen by (all) the other passes
			c.h.Pos = int(math.Round(base.Staves[c.h.Staff].Pos(c.h.X, c.h.Y)))
			out = append(out, c.h)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].X < out[j].X })
	base.Heads = out
	// re-apply 8va ranges to any added heads
	for k, rg := range base.Ottava {
		for _, h := range base.Heads {
			if h.Staff == base.OttavaSt[k] && h.X >= rg[0] && h.X <= rg[1] {
				h.Octave = 1
			}
		}
	}
	base.findTies()
	return base
}

func argmaxInt(m map[int]int, def int) int {
	best, bn := def, m[def]
	for k, n := range m {
		if n > bn {
			best, bn = k, n
		}
	}
	return best
}
