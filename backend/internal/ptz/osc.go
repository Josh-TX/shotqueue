package ptz

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// Minimal OSC 1.0 message codec: just int32 and float32 args, which is all the Tail 2 uses.

func oscPad(n int) int { return (4 - n%4) % 4 }

func oscString(b []byte, s string) []byte {
	b = append(b, s...)
	b = append(b, 0)
	for i := 0; i < oscPad(len(s)+1); i++ {
		b = append(b, 0)
	}
	return b
}

// encodeOSC builds a message with int32 args.
func encodeOSC(addr string, ints ...int32) []byte {
	b := oscString(nil, addr)
	tags := ","
	for range ints {
		tags += "i"
	}
	b = oscString(b, tags)
	for _, v := range ints {
		b = binary.BigEndian.AppendUint32(b, uint32(v))
	}
	return b
}

func readOSCString(b []byte) (string, []byte, error) {
	for i, c := range b {
		if c == 0 {
			n := i + 1 + oscPad(i+1)
			if n > len(b) {
				return "", nil, errors.New("osc: truncated string")
			}
			return string(b[:i]), b[n:], nil
		}
	}
	return "", nil, errors.New("osc: unterminated string")
}

// decodeOSC parses a message; int and float args are both returned as float64.
func decodeOSC(b []byte) (addr string, args []float64, err error) {
	addr, b, err = readOSCString(b)
	if err != nil {
		return
	}
	var tags string
	tags, b, err = readOSCString(b)
	if err != nil || len(tags) == 0 || tags[0] != ',' {
		return addr, nil, err
	}
	for _, t := range tags[1:] {
		switch t {
		case 'i':
			if len(b) < 4 {
				return addr, nil, errors.New("osc: truncated int")
			}
			args = append(args, float64(int32(binary.BigEndian.Uint32(b))))
			b = b[4:]
		case 'f':
			if len(b) < 4 {
				return addr, nil, errors.New("osc: truncated float")
			}
			args = append(args, float64(math.Float32frombits(binary.BigEndian.Uint32(b))))
			b = b[4:]
		default:
			return addr, nil, fmt.Errorf("osc: unsupported type tag %q", t)
		}
	}
	return addr, args, nil
}
