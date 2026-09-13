// Diagnostic: drives sofie-atem-connection's exact wire protocol (from
// src/lib/atemSocketChild.ts) directly, no deps beyond Go's net package.
// Prints live/preview only when it changes, so a CUT press is visible as a new line.
// Ported from js-atem-listener-v2/main.js; intentionally does NOT send
// RETRANSMIT_REQUEST on a gap (unlike go-atem-listener) - it just re-ACKs the
// last good packet and lets the switcher's own retransmit timer handle it.
package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const (
	port        = 9910
	maxPacketID = 1 << 15

	handshakeRetryInterval = 2 * time.Second
	establishedTimeout     = 10 * time.Second // no packets received for this long => connection considered dead
)

const (
	flagAckRequest        uint16 = 0x01
	flagNewSessionID      uint16 = 0x02
	flagRetransmitRequest uint16 = 0x08
	flagAckReply          uint16 = 0x10
)

var commandConnectHello = []byte{
	0x10, 0x14, 0x53, 0xab, 0x00, 0x00, 0x00, 0x00, 0x00, 0x3a, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00,
}

type command struct {
	name    string
	payload []byte
}

type listener struct {
	conn       *net.UDPConn
	remoteAddr *net.UDPAddr

	mu                   sync.Mutex
	sessionID            uint16
	lastReceivedPacketID uint16
	established          bool
	highestSeen          uint16
	lastTallyKey         string
}

func (l *listener) send(buf []byte) {
	l.conn.WriteToUDP(buf, l.remoteAddr)
}

func (l *listener) sendAck(packetID uint16) {
	buf := make([]byte, 12)
	binary.BigEndian.PutUint16(buf[0:2], (flagAckReply<<11)|12)
	l.mu.Lock()
	sessionID := l.sessionID
	l.mu.Unlock()
	binary.BigEndian.PutUint16(buf[2:4], sessionID)
	binary.BigEndian.PutUint16(buf[4:6], packetID)
	l.send(buf)
}

func isPacketCoveredByAck(ackID, packetID uint16) bool {
	const tolerance = maxPacketID / 2
	shortlyBefore := packetID < ackID && packetID+tolerance > ackID
	shortlyAfter := packetID > ackID && packetID < ackID+tolerance
	beforeWrap := packetID > ackID+tolerance
	return packetID == ackID || ((shortlyBefore || beforeWrap) && !shortlyAfter)
}

func parseCommands(payload []byte) []command {
	var commands []command
	offset := 0
	for offset+8 <= len(payload) {
		length := int(binary.BigEndian.Uint16(payload[offset : offset+2]))
		if length < 8 || offset+length > len(payload) {
			break
		}
		name := string(payload[offset+4 : offset+8])
		commands = append(commands, command{name: name, payload: payload[offset+8 : offset+length]})
		offset += length
	}
	return commands
}

func (l *listener) handleTlSr(packetID uint16, payload []byte) {
	if len(payload) < 2 {
		return
	}
	count := int(binary.BigEndian.Uint16(payload[0:2]))
	live := []uint16{}
	preview := []uint16{}
	for i := 0; i < count; i++ {
		off := 2 + i*3
		if off+3 > len(payload) {
			break
		}
		source := binary.BigEndian.Uint16(payload[off : off+2])
		flags := payload[off+2]
		if flags&0x01 != 0 {
			live = append(live, source)
		}
		if flags&0x02 != 0 {
			preview = append(preview, source)
		}
	}

	keyBytes, _ := json.Marshal(struct {
		Live    []uint16 `json:"live"`
		Preview []uint16 `json:"preview"`
	}{live, preview})
	key := string(keyBytes)

	l.mu.Lock()
	if key == l.lastTallyKey {
		l.mu.Unlock()
		return
	}
	l.lastTallyKey = key
	l.mu.Unlock()

	fmt.Printf("[tally CHANGED] packetId=%d live=%s preview=%s\n", packetID, mustJSON(live), mustJSON(preview))
}

func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func (l *listener) onMessage(packet []byte) {
	if len(packet) < 12 {
		return
	}
	length := binary.BigEndian.Uint16(packet[0:2]) & 0x07ff
	if int(length) != len(packet) {
		return
	}
	flags := uint16(packet[0]) >> 3
	sessionID := binary.BigEndian.Uint16(packet[2:4])
	remotePacketID := binary.BigEndian.Uint16(packet[10:12])

	l.mu.Lock()
	l.sessionID = sessionID
	l.mu.Unlock()

	if flags&flagNewSessionID != 0 {
		l.mu.Lock()
		l.established = true
		l.lastReceivedPacketID = remotePacketID
		l.mu.Unlock()
		fmt.Printf("[handshake] established session=0x%x\n", sessionID)
		l.sendAck(remotePacketID)
		return
	}

	l.mu.Lock()
	established := l.established
	l.mu.Unlock()
	if !established {
		return
	}

	if flags&flagAckRequest != 0 {
		l.mu.Lock()
		expected := (l.lastReceivedPacketID + 1) % maxPacketID
		l.mu.Unlock()

		if remotePacketID == expected {
			l.mu.Lock()
			l.lastReceivedPacketID = remotePacketID
			if remotePacketID > l.highestSeen {
				l.highestSeen = remotePacketID
			}
			l.mu.Unlock()
			l.sendAck(remotePacketID)
			if length > 12 {
				for _, cmd := range parseCommands(packet[12:]) {
					if cmd.name == "TlSr" {
						l.handleTlSr(remotePacketID, cmd.payload)
					}
				}
			}
		} else {
			l.mu.Lock()
			lastReceived := l.lastReceivedPacketID
			l.mu.Unlock()
			if isPacketCoveredByAck(lastReceived, remotePacketID) {
				l.sendAck(lastReceived)
			}
		}
	}
}

func main() {
	address := "192.168.10.240"
	if len(os.Args) > 1 {
		address = os.Args[1]
	}

	remoteAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", address, port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[error] %v\n", err)
		os.Exit(1)
	}

	conn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[error] %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	l := &listener{conn: conn, remoteAddr: remoteAddr}

	fmt.Printf("connecting to %s:%d ... (press CUT on the switcher after \"established\" to test)\n", address, port)
	l.send(commandConnectHello)

	helloTicker := time.NewTicker(2 * time.Second)
	defer helloTicker.Stop()
	go func() {
		for range helloTicker.C {
			l.mu.Lock()
			established := l.established
			l.mu.Unlock()
			if !established {
				l.send(commandConnectHello)
			}
		}
	}()

	statusTicker := time.NewTicker(5 * time.Second)
	defer statusTicker.Stop()
	go func() {
		for range statusTicker.C {
			l.mu.Lock()
			established := l.established
			highestSeen := l.highestSeen
			l.mu.Unlock()
			fmt.Printf("[status] established=%v highestPacketId=%d\n", established, highestSeen)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGINT)
	go func() {
		<-sigCh
		os.Exit(0)
	}()

	buf := make([]byte, 2048)
	for {
		l.mu.Lock()
		established := l.established
		l.mu.Unlock()

		deadline := handshakeRetryInterval
		if established {
			deadline = establishedTimeout
		}
		conn.SetReadDeadline(time.Now().Add(deadline))

		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				if established {
					fmt.Printf("[status] no packets for %s, marking dead and re-sending hello\n", establishedTimeout)
					l.mu.Lock()
					l.established = false
					l.mu.Unlock()
				}
				l.send(commandConnectHello)
				continue
			}
			fmt.Fprintf(os.Stderr, "[error] %v\n", err)
			continue
		}
		packet := make([]byte, n)
		copy(packet, buf[:n])
		l.onMessage(packet)
	}
}
