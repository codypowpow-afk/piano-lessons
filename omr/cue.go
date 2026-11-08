package omr

import "math"

// Cue-size notes (cadenzas, ornamental runs) are about 60% of a normal
// notehead, so the main detector skips them. Accepting every small blob lets
// in dots, letters and pieces of accidentals, so a small notehead is only
// accepted with structural evidence: a stem that touches its edge and ends in
// a beam, and at least three such noteheads hanging from the same beam.

var cueNotes = true

var cueLoose = true
var cueFullRes = true
var cueMinGroup = 4

func (pr *PageResult) findCueHeads() {
	if !cueNotes {
		return
	}
	if cueFullRes {
		if pr.full != nil && pr.cuePass {
			pr.findCueHeadsFull()
		}
		return
	}
	sp := pr.Space
	o := keepRuns(pr.Clean, int(sp*0.24+0.5), true)
	o = keepRuns(o, int(sp*0.3+0.5), false)
	_, comps := label(o)
	cleanLab, _ := label(pr.Clean)
	type cand struct {
		h      *Head
		beam   int32
		ex, ey float64 // where the stem meets the beam
	}
	var cands []cand
	for _, c := range comps {
		w, h := float64(c.W()), float64(c.H())
		if w < sp*0.45 || w > sp*0.85 || h < sp*0.42 || h > sp*0.82 {
			continue
		}
		if float64(c.Area)/(w*h) < 0.68 {
			continue
		}
		cx, cy := float64(c.X0+c.X1)/2, float64(c.Y0+c.Y1)/2
		// skip anything already taken by a normal notehead
		taken := false
		for _, o := range pr.Heads {
			if math.Abs(o.X-cx) < sp*0.7 && math.Abs(o.Y-cy) < sp*0.6 {
				taken = true
				break
			}
		}
		if taken {
			continue
		}
		st := pr.staffAt(cx, cy)
		if st < 0 {
			continue
		}
		// a stem touching the left or right edge whose far end is a beam
		// (an accidental just left of the head also gives a tall stroke, so
		// every candidate column is tried)
		hw := w / 2
		beamAt := func(x, y int) (int32, bool) {
			wd := 0
			for dx := -int(sp * 1.5); dx <= int(sp*1.5); dx++ {
				if pr.Clean.At(x+dx, y) == 1 || pr.Clean.At(x+dx, y+1) == 1 || pr.Clean.At(x+dx, y-1) == 1 {
					wd++
				}
			}
			if float64(wd) < sp*1.2 {
				return 0, false
			}
			// the beam is the widest run of ink at the stem's end
			var bestL int32
			bestRun := 0
			for dy := -3; dy <= 3; dy++ {
				for dx := -1; dx <= 1; dx++ {
					yy, xx := y+dy, x+dx
					if yy < 0 || yy >= pr.Clean.H || xx < 0 || xx >= pr.Clean.W {
						continue
					}
					if l := cleanLab[yy*pr.Clean.W+xx]; l != 0 {
						if r := pr.hrun(xx, yy); r > bestRun {
							bestRun, bestL = r, l
						}
					}
				}
			}
			if bestL != 0 {
				return bestL, true
			}
			return 0, false
		}
		var beam int32
		found, up, stemX := false, false, 0
		for _, ex := range []float64{cx + hw, cx - hw} {
			for x := int(ex - sp*0.12); x <= int(ex+sp*0.12) && !found; x++ {
				for y := int(cy - sp*0.2); y <= int(cy+sp*0.2) && !found; y += 2 {
					top, bot := pr.vrun(x, y)
					if float64(bot-top+1) < sp*1.1 || float64(bot-top+1) > sp*5 {
						continue
					}
					if l, ok := beamAt(x, top+1); ok && cy-float64(top) > sp*0.9 {
						beam, up, stemX, found = l, true, x, true
					} else if l, ok := beamAt(x, bot-1); ok && float64(bot)-cy > sp*0.9 {
						beam, up, stemX, found = l, false, x, true
					} else if cueLoose && cy-float64(top) > sp*1.3 && float64(bot)-cy < sp*0.6 {
						// a clear up-stem; the beam test failed on broken ink
						up, stemX, found = true, x, true
					} else if cueLoose && float64(bot)-cy > sp*1.3 && cy-float64(top) < sp*0.6 {
						up, stemX, found = false, x, true
					}
				}
			}
		}
		if !found {
			continue
		}
		bestX := stemX
		endY := 0
		{
			t, b := pr.vrun(stemX, int(cy))
			if up {
				endY = t
			} else {
				endY = b
			}
		}
		hd := &Head{X: cx, Y: cy, Staff: st, Small: true, W: w, StemX: float64(bestX), Pass: passCue}
		if up {
			hd.StemDir = 1
			hd.OnsetX = hd.StemX - sp*0.45
		} else {
			hd.StemDir = -1
			hd.OnsetX = hd.StemX + sp*0.45
		}
		cands = append(cands, cand{hd, beam, float64(stemX), float64(endY)})
	}
	// group noteheads whose stems end on the same beam: chain stem ends that
	// are close horizontally and at nearly the same height
	parent := make([]int, len(cands))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	for i := range cands {
		for j := i + 1; j < len(cands); j++ {
			a, b := cands[i], cands[j]
			if a.h.Staff == b.h.Staff && a.h.StemDir == b.h.StemDir &&
				math.Abs(a.ex-b.ex) < sp*3.5 && math.Abs(a.ey-b.ey) < sp*1.0 {
				parent[find(i)] = find(j)
			}
		}
	}
	size := map[int]int{}
	for i := range cands {
		size[find(i)]++
	}
	for i, c := range cands {
		if size[find(i)] >= 3 {
			pr.Heads = append(pr.Heads, c.h)
		}
	}
}

// findCueHeadsFull runs the cue-note search on the original scan, staff by
// staff, where the thin stems and beams of small notes are still intact.
func (pr *PageResult) findCueHeadsFull() {
	fb := pr.full
	f := pr.fscale
	spF := pr.Space * f
	thickF := int(float64(pr.Thick)*f + 0.5)
	for si, st := range pr.Staves {
		x0 := int(float64(st.X0) * f)
		x1 := int(float64(st.X1) * f)
		y0 := int(st.Top(float64(st.X0+st.X1)/2)*f - spF*6.5)
		y1 := int(st.Bottom(float64(st.X0+st.X1)/2)*f + spF*6.5)
		x0, y0 = max(0, x0), max(0, y0)
		x1, y1 = min(fb.W-1, x1), min(fb.H-1, y1)
		w, h := x1-x0+1, y1-y0+1
		if w < 10 || h < 10 {
			continue
		}
		crop := &Bitmap{W: w, H: h, Pix: make([]uint8, w*h)}
		for y := 0; y < h; y++ {
			copy(crop.Pix[y*w:(y+1)*w], fb.Pix[(y+y0)*fb.W+x0:(y+y0)*fb.W+x0+w])
		}
		// remove this staff's lines (vertical runs no thicker than a line)
		for x := 0; x < w; x++ {
			wx := float64(x+x0) / f
			for l := 0; l < 5; l++ {
				yc := int(st.LineY(l, wx)*f) - y0
				ya := -1
				for dy := -thickF; dy <= thickF; dy++ {
					if crop.At(x, yc+dy) == 1 {
						ya = yc + dy
						break
					}
				}
				if ya < 0 {
					continue
				}
				s0, e0 := ya, ya
				for s0 > 0 && crop.At(x, s0-1) == 1 {
					s0--
				}
				for e0 < h-1 && crop.At(x, e0+1) == 1 {
					e0++
				}
				if e0-s0+1 <= thickF+3 {
					for y := s0; y <= e0; y++ {
						crop.Pix[y*w+x] = 0
					}
				}
			}
		}
		o := keepRuns(crop, int(spF*0.24+0.5), true)
		o = keepRuns(o, int(spF*0.3+0.5), false)
		_, comps := label(o)
		vrun := func(x, y int) (int, int) {
			if crop.At(x, y) == 0 {
				return y, y - 1
			}
			maxGap := thickF + 3
			run := func(dir int) int {
				cx, cy, last, gap := x, y, y, 0
				for {
					ny := cy + dir
					if ny < 0 || ny >= h {
						return last
					}
					switch {
					case crop.At(cx, ny) == 1:
					case crop.At(cx-1, ny) == 1:
						cx--
					case crop.At(cx+1, ny) == 1:
						cx++
					default:
						gap++
						if gap > maxGap {
							return last
						}
						cy = ny
						continue
					}
					if abs(cx-x) > int(spF*0.12)+1 {
						return last
					}
					gap = 0
					cy, last = ny, ny
				}
			}
			return run(-1), run(1)
		}
		wideAt := func(x, y int) bool {
			n := 0
			for dx := -int(spF * 1.5); dx <= int(spF*1.5); dx++ {
				if crop.At(x+dx, y) == 1 || crop.At(x+dx, y+2) == 1 || crop.At(x+dx, y-2) == 1 {
					n++
				}
			}
			return float64(n) >= spF*1.2
		}
		type cand struct {
			h      *Head
			ex, ey float64
		}
		var cands []cand
		olab, _ := label(o)
		type box struct{ x0, x1, y0, y1, bx0, bx1 int }
		var boxes []box
		for _, c := range comps {
			cw, ch := float64(c.W()), float64(c.H())
			if ch < spF*0.42 || ch > spF*0.85 || cw < spF*0.45 || cw > spF*1.8 {
				continue
			}
			if cw <= spF*0.85 {
				if float64(c.Area)/(cw*ch) >= 0.68 {
					boxes = append(boxes, box{c.X0, c.X1, c.Y0, c.Y1, c.X0, c.X1})
				}
				continue
			}
			// a notehead fused with its accidental: the densest head-wide window
			win := int(spF * 0.62)
			best, bi := 0, -1
			for xs := c.X0; xs+win <= c.X1; xs++ {
				n := 0
				for x := xs; x < xs+win; x++ {
					for y := c.Y0; y < c.Y1; y++ {
						if olab[y*w+x] == int32(c.ID) {
							n++
						}
					}
				}
				if n > best {
					best, bi = n, xs
				}
			}
			if bi >= 0 && float64(best) >= float64(win)*ch*0.68 {
				boxes = append(boxes, box{bi, bi + win, c.Y0, c.Y1, c.X0, c.X1})
			}
		}
		for _, c := range boxes {
			cw := float64(c.x1 - c.x0)
			cx, cy := float64(c.x0+c.x1)/2, float64(c.y0+c.y1)/2
			wx, wy := (cx+float64(x0))/f, (cy+float64(y0))/f
			if pr.staffAt(wx, wy) != si {
				continue
			}
			taken := false
			for _, o := range pr.Heads {
				// a small notehead the main detector found is re-judged here
				if !o.Small && math.Abs(o.X-wx) < pr.Space*0.7 && math.Abs(o.Y-wy) < pr.Space*0.6 {
					taken = true
					break
				}
			}
			if taken {
				continue
			}
			found, up, stemX, endY := false, false, 0, 0
			// search the whole blob (a fused accidental shifts the estimated
			// centre), right side first
			lo, hi := float64(c.bx0)-spF*0.3, float64(c.bx1)+spF*0.3
			for _, rng := range [][2]float64{{cx, hi}, {lo, cx}} {
				for x := int(rng[1]); x >= int(rng[0]) && !found; x-- {
					for y := int(cy - spF*0.2); y <= int(cy+spF*0.2) && !found; y += 3 {
						top, bot := vrun(x, y)
						l := float64(bot - top + 1)
						if l < spF*1.1 || l > spF*5 {
							continue
						}
						// the beam can sit a little inside the traced end (the
						// trace may run on into an 8va dot or a slur)
						beamNear := func(from, dir int) (int, bool) {
							for k := 0; k <= int(spF*0.6); k += 2 {
								if wideAt(x, from+dir*k) {
									return from + dir*k, true
								}
							}
							return 0, false
						}
						if cy-float64(top) > spF*0.9 {
							if by, ok := beamNear(top+2, 1); ok && cy-float64(by) > spF*0.8 {
								up, stemX, endY, found = true, x, by, true
							}
						}
						if !found && float64(bot)-cy > spF*0.9 {
							if by, ok := beamNear(bot-2, -1); ok && float64(by)-cy > spF*0.8 {
								up, stemX, endY, found = false, x, by, true
							}
						}
					}
				}
			}
			if !found {
				continue
			}
			// re-centre the head against its stem (up-stem: head to the left)
			if up {
				cx = math.Min(cx, float64(stemX)-spF*0.3)
			} else {
				cx = math.Max(cx, float64(stemX)+spF*0.3)
			}
			wx = (cx + float64(x0)) / f
			hd := &Head{X: wx, Y: wy, Staff: si, Small: true, W: cw / f, StemX: (float64(stemX + x0)) / f, Pass: passCue}
			if up {
				hd.StemDir = 1
				hd.OnsetX = hd.StemX - pr.Space*0.45
			} else {
				hd.StemDir = -1
				hd.OnsetX = hd.StemX + pr.Space*0.45
			}
			cands = append(cands, cand{hd, float64(stemX), float64(endY)})
		}
		parent := make([]int, len(cands))
		for i := range parent {
			parent[i] = i
		}
		var find func(int) int
		find = func(i int) int {
			for parent[i] != i {
				parent[i] = parent[parent[i]]
				i = parent[i]
			}
			return i
		}
		for i := range cands {
			for j := i + 1; j < len(cands); j++ {
				a, b := cands[i], cands[j]
				if a.h.StemDir == b.h.StemDir && math.Abs(a.ex-b.ex) < spF*3.5 && math.Abs(a.ey-b.ey) < spF*1.0 {
					parent[find(i)] = find(j)
				}
			}
		}
		size := map[int]int{}
		for i := range cands {
			size[find(i)]++
		}
		for i, c := range cands {
			if size[find(i)] >= cueMinGroup {
				// replace a small head the main detector put here
				var keep []*Head
				for _, o := range pr.Heads {
					if o.Small && o.Pass != passCue && math.Abs(o.X-c.h.X) < pr.Space*0.7 && math.Abs(o.Y-c.h.Y) < pr.Space*0.6 {
						continue
					}
					keep = append(keep, o)
				}
				pr.Heads = append(keep, c.h)
			}
		}
	}
}
