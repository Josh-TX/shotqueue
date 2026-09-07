package main

import "encoding/binary"

type tallyEntry struct {
	program bool
	preview bool
}

type switcherState struct {
	tally map[uint16]tallyEntry
}

func newSwitcherState() *switcherState {
	return &switcherState{tally: make(map[uint16]tallyEntry)}
}

// applyCommand updates state from a single parsed command. Returns true if tally changed.
func (s *switcherState) applyCommand(cmd command) bool {
	if cmd.name != "TlSr" {
		return false
	}
	if len(cmd.payload) < 2 {
		return false
	}
	count := int(binary.BigEndian.Uint16(cmd.payload[0:2]))
	next := make(map[uint16]tallyEntry)
	for i := 0; i < count; i++ {
		off := 2 + i*3
		if off+3 > len(cmd.payload) {
			break
		}
		source := binary.BigEndian.Uint16(cmd.payload[off : off+2])
		flags := cmd.payload[off+2]
		next[source] = tallyEntry{
			program: flags&0x01 != 0,
			preview: flags&0x02 != 0,
		}
	}

	changed := !tallyEqual(s.tally, next)
	s.tally = next
	return changed
}

func tallyEqual(a, b map[uint16]tallyEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func (s *switcherState) liveAndPreview() (live, preview []uint16) {
	for source, t := range s.tally {
		if t.program {
			live = append(live, source)
		}
		if t.preview {
			preview = append(preview, source)
		}
	}
	return live, preview
}
