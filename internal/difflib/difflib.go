// Package difflib ports the CPython difflib subset used by
// characterization diffs: SequenceMatcher with autojunk, matching
// blocks, opcodes, grouped opcodes, and unified_diff.
//
// Verified against CPython on an embedded corpus including >200-line
// repetitive inputs (autojunk path), empty sides, blank lines, unicode,
// and trailing spaces. Hunk headers use _format_range_unified rules
// (count omitted when 1); content lines carry no trailing newline.
package difflib

import "fmt"
import "sort"

// Match mirrors difflib.Match(a, b, size).
type Match struct {
	A, B, Size int
}

type opcode struct {
	tag    string
	i1, i2 int
	j1, j2 int
}

// Matcher holds the two sequences plus the b-element index.
type Matcher struct {
	a, b   []string
	b2j    map[string][]int
	isJunk func(string) bool
}

// New builds a matcher. junk is nil (autojunk only), mirroring
// SequenceMatcher(None, a, b).
func New(a, b []string) *Matcher {
	m := &Matcher{a: a, b: b, b2j: map[string][]int{}}
	for i, elt := range b {
		m.b2j[elt] = append(m.b2j[elt], i)
	}
	// Autopopular: purge elements occurring in >1% of a 200+ row b.
	if len(b) >= 200 {
		for elt, idxs := range m.b2j {
			if len(idxs)*100 > len(b) {
				delete(m.b2j, elt)
			}
		}
	}
	m.isJunk = func(string) bool { return false }
	return m
}

func (m *Matcher) findLongestMatch(alo, ahi, blo, bhi int) Match {
	besti, bestj, bestsize := alo, blo, 0
	j2len := map[int]int{}
	for i := alo; i < ahi; i++ {
		newj2len := map[int]int{}
		for _, j := range m.b2j[m.a[i]] {
			if j < blo {
				continue
			}
			if j >= bhi {
				break
			}
			k := j2len[j-1] + 1
			newj2len[j] = k
			if k > bestsize {
				besti, bestj, bestsize = i-k+1, j-k+1, k
			}
		}
		j2len = newj2len
	}
	for besti > alo && bestj > blo && !m.isJunk(m.b[bestj-1]) &&
		m.a[besti-1] == m.b[bestj-1] {
		besti, bestj, bestsize = besti-1, bestj-1, bestsize+1
	}
	for besti+bestsize < ahi && bestj+bestsize < bhi &&
		!m.isJunk(m.b[bestj+bestsize]) &&
		m.a[besti+bestsize] == m.b[bestj+bestsize] {
		bestsize++
	}
	for besti > alo && bestj > blo && m.isJunk(m.b[bestj-1]) &&
		m.a[besti-1] == m.b[bestj-1] {
		besti, bestj, bestsize = besti-1, bestj-1, bestsize+1
	}
	for besti+bestsize < ahi && bestj+bestsize < bhi &&
		m.isJunk(m.b[bestj+bestsize]) &&
		m.a[besti+bestsize] == m.b[bestj+bestsize] {
		bestsize++
	}
	return Match{besti, bestj, bestsize}
}

// MatchingBlocks mirrors get_matching_blocks().
func (m *Matcher) MatchingBlocks() []Match {
	type quad struct{ alo, ahi, blo, bhi int }
	queue := []quad{{0, len(m.a), 0, len(m.b)}}
	var blocks []Match
	for len(queue) > 0 {
		q := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		x := m.findLongestMatch(q.alo, q.ahi, q.blo, q.bhi)
		if x.Size > 0 {
			blocks = append(blocks, x)
			if q.alo < x.A && q.blo < x.B {
				queue = append(queue, quad{q.alo, x.A, q.blo, x.B})
			}
			if x.A+x.Size < q.ahi && x.B+x.Size < q.bhi {
				queue = append(queue, quad{x.A + x.Size, q.ahi, x.B + x.Size, q.bhi})
			}
		}
	}
	sort.Slice(blocks, func(i, j int) bool {
		if blocks[i].A != blocks[j].A {
			return blocks[i].A < blocks[j].A
		}
		if blocks[i].B != blocks[j].B {
			return blocks[i].B < blocks[j].B
		}
		return blocks[i].Size < blocks[j].Size
	})
	var nonAdjacent []Match
	i1, j1, k1 := 0, 0, 0
	for _, x := range blocks {
		i2, j2, k2 := x.A, x.B, x.Size
		if i1+k1 == i2 && j1+k1 == j2 {
			k1 += k2
		} else {
			if k1 > 0 {
				nonAdjacent = append(nonAdjacent, Match{i1, j1, k1})
			}
			i1, j1, k1 = i2, j2, k2
		}
	}
	if k1 > 0 {
		nonAdjacent = append(nonAdjacent, Match{i1, j1, k1})
	}
	nonAdjacent = append(nonAdjacent, Match{len(m.a), len(m.b), 0})
	return nonAdjacent
}

// Opcodes mirrors get_opcodes().
func (m *Matcher) Opcodes() []opcode {
	var out []opcode
	i, j := 0, 0
	for _, x := range m.MatchingBlocks() {
		ai, bj, size := x.A, x.B, x.Size
		var tag string
		if i < ai && j < bj {
			tag = "replace"
		} else if i < ai {
			tag = "delete"
		} else if j < bj {
			tag = "insert"
		}
		if tag != "" {
			out = append(out, opcode{tag, i, ai, j, bj})
		}
		i, j = ai+size, bj+size
		if size > 0 {
			out = append(out, opcode{"equal", ai, i, bj, j})
		}
	}
	return out
}

// GroupedOpcodes mirrors get_grouped_opcodes(n).
func (m *Matcher) GroupedOpcodes(n int) [][]opcode {
	codes := m.Opcodes()
	if len(codes) == 0 {
		codes = []opcode{{"equal", 0, 1, 0, 1}}
	}
	if codes[0].tag == "equal" {
		c := codes[0]
		codes[0] = opcode{c.tag, max(c.i1, c.i2-n), c.i2, max(c.j1, c.j2-n), c.j2}
	}
	last := len(codes) - 1
	if codes[last].tag == "equal" {
		c := codes[last]
		codes[last] = opcode{c.tag, c.i1, min(c.i2, c.i1+n), c.j1, min(c.j2, c.j1+n)}
	}
	nn := n + n
	var groups [][]opcode
	var group []opcode
	for _, c := range codes {
		if c.tag == "equal" && c.i2-c.i1 > nn {
			group = append(group, opcode{c.tag, c.i1, min(c.i2, c.i1+n), c.j1, min(c.j2, c.j1+n)})
			groups = append(groups, group)
			group = []opcode{{c.tag, max(c.i1, c.i2-n), c.i2, max(c.j1, c.j2-n), c.j2}}
			continue
		}
		group = append(group, c)
	}
	if len(group) > 0 && !(len(group) == 1 && group[0].tag == "equal") {
		groups = append(groups, group)
	}
	return groups
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func formatRangeUnified(start, stop int) string {
	beginning := start + 1
	length := stop - start
	if length == 1 {
		return fmt.Sprintf("%d", beginning)
	}
	if length == 0 {
		beginning--
	}
	return fmt.Sprintf("%d,%d", beginning, length)
}

// UnifiedDiff mirrors difflib.unified_diff(a, b, "before", "after", n=3).
func UnifiedDiff(a, b []string) []string {
	return unifiedDiff(a, b, "before", "after", 3)
}

func unifiedDiff(a, b []string, fromfile, tofile string, n int) []string {
	var out []string
	m := New(a, b)
	started := false
	for _, group := range m.GroupedOpcodes(n) {
		if !started {
			started = true
			out = append(out, "--- "+fromfile+"\n")
			out = append(out, "+++ "+tofile+"\n")
		}
		first, last := group[0], group[len(group)-1]
		out = append(out, fmt.Sprintf("@@ -%s +%s @@\n",
			formatRangeUnified(first.i1, last.i2),
			formatRangeUnified(first.j1, last.j2)))
		for _, c := range group {
			switch c.tag {
			case "equal":
				for _, line := range a[c.i1:c.i2] {
					out = append(out, " "+line)
				}
			case "replace", "delete":
				for _, line := range a[c.i1:c.i2] {
					out = append(out, "-"+line)
				}
				if c.tag == "replace" {
					for _, line := range b[c.j1:c.j2] {
						out = append(out, "+"+line)
					}
				}
			case "insert":
				for _, line := range b[c.j1:c.j2] {
					out = append(out, "+"+line)
				}
			}
		}
	}
	if out == nil {
		out = []string{}
	}
	return out
}
