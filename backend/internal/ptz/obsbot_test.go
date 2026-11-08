package ptz

import (
	"math"
	"net"
	"strconv"
	"testing"
	"time"
)

func TestOSCRoundTrip(t *testing.T) {
	addr, args, err := decodeOSC(encodeOSC("/a/b", 1, -2, 300))
	if err != nil || addr != "/a/b" || len(args) != 3 || args[1] != -2 || args[2] != 300 {
		t.Fatalf("got %q %v %v", addr, args, err)
	}
}

// Fake Tail 2 on loopback: answers Connected with [pitch, yaw] floats to port 57120, and applies
// SetGimMotorDegree immediately.
func TestObsbotMoveAndRead(t *testing.T) {
	cam, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer cam.Close()
	pitch, yaw := float32(0), float32(0)
	go func() {
		buf := make([]byte, 2048)
		for {
			n, from, err := cam.ReadFromUDP(buf)
			if err != nil {
				return
			}
			addr, args, _ := decodeOSC(buf[:n])
			switch addr {
			case addrSetDegree:
				yaw, pitch = float32(args[1]), float32(args[2])
			case addrConnected:
				reply := oscString(nil, addrPosResp)
				reply = oscString(reply, ",ff")
				for _, f := range []float32{pitch, yaw} {
					b := make([]byte, 4)
					u := float32bits(f)
					b[0], b[1], b[2], b[3] = byte(u>>24), byte(u>>16), byte(u>>8), byte(u)
					reply = append(reply, b...)
				}
				cam.WriteToUDP(reply, &net.UDPAddr{IP: from.IP, Port: obsbotReplyPort})
			}
		}
	}()

	port := cam.LocalAddr().(*net.UDPAddr).Port
	c := NewObsbot("127.0.0.1", itoa(port))
	if err := c.SetPositionRaw(Position{Pan: 200, Tilt: -10, Zoom: 50}); err != nil { // pan clamps to 159
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	pos, err := c.GetPosition()
	if err != nil {
		t.Fatal(err)
	}
	if pos != (Position{Pan: 159, Tilt: -10}) {
		t.Fatalf("got %+v", pos)
	}
	if !c.CloseEnough(Position{Pan: 158, Tilt: -10}, pos) || c.CloseEnough(Position{Pan: 150, Tilt: -10}, pos) {
		t.Fatal("CloseEnough wrong")
	}
}

func float32bits(f float32) uint32 { return math.Float32bits(f) }
func itoa(i int) string            { return strconv.Itoa(i) }
