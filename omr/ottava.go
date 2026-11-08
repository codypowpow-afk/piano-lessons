package omr

import (
	"math"
	"sort"
)

// findOttava detects "8 . . . . . . loco" (8va) lines above a staff: a short
// glyph followed by a chain of evenly spaced dots. Heads on that staff
// between the 8 and the last dot sound an octave higher.
var ottavaContinue = true

func (pr *PageResult) findOttava() {
	sp := pr.Space
	_, comps := label(pr.Clean)
	type dot struct{ x, y float64 }
	var dots []dot
	for _, c := range comps {
		if float64(c.W()) <= sp*0.5 && float64(c.H()) <= sp*0.5 && c.Area >= 3 {
			dots = append(dots, dot{float64(c.X0+c.X1) / 2, float64(c.Y0+c.Y1) / 2})
		}
	}
	sort.Slice(dots, func(i, j int) bool { return dots[i].x < dots[j].x })
	used := make([]bool, len(dots))
	var ottY []float64
	for i := range dots {
		if used[i] {
			continue
		}
		chain := []int{i}
		last := dots[i]
		for j := i + 1; j < len(dots); j++ {
			d := dots[j]
			if d.x-last.x > sp*1.3 {
				break
			}
			if used[j] || math.Abs(d.y-last.y) > sp*0.25 || d.x-last.x < sp*0.25 {
				continue
			}
			chain = append(chain, j)
			last = d
		}
		if len(chain) >= 3 && ottavaContinue {
			// a short run of dots right after an 8va line extends it
			cx0, cx1, cy := dots[chain[0]].x, dots[chain[len(chain)-1]].x, dots[chain[0]].y
			ext := false
			for k, rg := range pr.Ottava {
				if cx0 > rg[1]-sp && cx0-rg[1] < sp*20 && math.Abs(cy-ottY[k]) < sp*0.8 {
					pr.Ottava[k][1] = math.Max(rg[1], cx1+sp*0.8)
					ext = true
				}
			}
			if ext {
				for _, k := range chain {
					used[k] = true
				}
				continue
			}
		}
		if len(chain) < 6 {
			continue
		}
		// evenly spaced?
		var gaps []float64
		for k := 1; k < len(chain); k++ {
			gaps = append(gaps, dots[chain[k]].x-dots[chain[k-1]].x)
		}
		mg := median(gaps)
		even := 0
		for _, g := range gaps {
			if math.Abs(g-mg) < mg*0.35 {
				even++
			}
		}
		if even < len(gaps)*7/10 {
			continue
		}
		for _, k := range chain {
			used[k] = true
		}
		x0, x1 := dots[chain[0]].x, dots[chain[len(chain)-1]].x
		y := dots[chain[0]].y
		// it must lie above a staff (the one it applies to)
		st := -1
		for si, s := range pr.Staves {
			top := s.Top(x0)
			if y < top-sp*0.8 && y > top-sp*6 && x0 >= float64(s.X0)-sp && x0 <= float64(s.X1) {
				st = si
			}
		}
		if st < 0 {
			continue
		}
		// an "8" just before the dots
		start := x0
		for _, c := range comps {
			cx, cy := float64(c.X0+c.X1)/2, float64(c.Y0+c.Y1)/2
			if cx < x0 && cx > x0-sp*3 && math.Abs(cy-y) < sp*1.2 &&
				float64(c.H()) > sp*0.7 && float64(c.H()) < sp*2 && float64(c.W()) < sp*1.6 {
				start = math.Min(start, float64(c.X0))
			}
		}
		if start == x0 {
			// no 8: a continuation of an 8va line that was broken by a gap?
			cont := false
			for k, rg := range pr.Ottava {
				if !ottavaContinue {
					break
				}
				if pr.OttavaSt[k] == st && x0 > rg[1]-sp && x0-rg[1] < sp*20 && math.Abs(y-ottY[k]) < sp*0.8 {
					pr.Ottava[k][1] = math.Max(rg[1], x1+sp*0.8)
					cont = true
				}
			}
			// a dotted line starting right at the beginning of a staff is an
			// 8va carried over from the previous line of music
			stS := pr.Staves[st]
			if !cont && len(chain) >= 6 && x0 < float64(stS.X0)+sp*9 {
				pr.Ottava = append(pr.Ottava, [2]float64{float64(stS.X0), x1 + sp*0.8})
				pr.OttavaSt = append(pr.OttavaSt, st)
				ottY = append(ottY, y)
				continue
			}
			if !cont {
				continue // no 8: probably a different dotted line
			}
			continue
		}
		pr.Ottava = append(pr.Ottava, [2]float64{start - sp*0.5, x1 + sp*0.8})
		pr.OttavaSt = append(pr.OttavaSt, st)
		ottY = append(ottY, y)
	}
	for k, r := range pr.Ottava {
		for _, h := range pr.Heads {
			if h.Staff == pr.OttavaSt[k] && h.X >= r[0] && h.X <= r[1] {
				h.Octave = 1
			}
		}
	}
}
