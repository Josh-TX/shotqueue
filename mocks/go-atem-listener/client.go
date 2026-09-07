package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sort"
	"time"
)

const (
	atemPort           = 9910
	handshakeTimeout   = 2 * time.Second
	establishedTimeout = 5 * time.Second // no packets received for this long => connection considered dead
)

type tallyOutput struct {
	Live    []uint16 `json:"live"`
	Preview []uint16 `json:"preview"`
}

func runForever(ip string) {
	backoff := time.Second
	for {
		err := connectAndRun(ip)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[go-atem-listener] disconnected: %v\n", err)
		}
		fmt.Fprintf(os.Stderr, "[go-atem-listener] reconnecting in %s\n", backoff)
		time.Sleep(backoff)
		if backoff < 8*time.Second {
			backoff *= 2
		}
	}
}

func connectAndRun(ip string) error {
	addr := &net.UDPAddr{IP: net.ParseIP(ip), Port: atemPort}
	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	sessionID, lastReceivedPacketID, err := handshake(conn)
	if err != nil {
		return fmt.Errorf("handshake: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[go-atem-listener] connected to %s (session %d)\n", ip, sessionID)

	state := newSwitcherState()
	var lastPrinted *tallyOutput

	buf := make([]byte, 2048)
	for {
		conn.SetReadDeadline(time.Now().Add(establishedTimeout))
		n, err := conn.Read(buf)
		if err != nil {
			return fmt.Errorf("no data from switcher for %s", establishedTimeout)
		}
		pkt := make([]byte, n)
		copy(pkt, buf[:n])
		h := parseHeader(pkt)

		if h.flags&flagAckRequest != 0 {
			expected := (lastReceivedPacketID + 1) % maxPacketID
			if h.packetID == expected {
				lastReceivedPacketID = h.packetID
				sendAck(conn, sessionID, h.packetID)

				if h.length > 12 {
					for _, cmd := range parseCommands(pkt[12:]) {
						if state.applyCommand(cmd) {
							printTallyIfChanged(state, &lastPrinted)
						}
					}
				}
			} else if isPacketCoveredByAck(lastReceivedPacketID, h.packetID) {
				sendAck(conn, sessionID, lastReceivedPacketID)
			} else {
				// Gap detected: ask the switcher to resend from where we left off.
				req := buildPacket(flagRetransmitReq, sessionID, 0, expected, 0, nil)
				conn.Write(req)
			}
		}
	}
}

func handshake(conn *net.UDPConn) (sessionID uint16, lastReceivedPacketID uint16, err error) {
	hello := buildPacket(flagNewSessionID, helloSessionID, 0, 0, 0, nil)

	buf := make([]byte, 2048)
	for attempt := 0; attempt < 5; attempt++ {
		if _, err := conn.Write(hello); err != nil {
			return 0, 0, err
		}
		conn.SetReadDeadline(time.Now().Add(handshakeTimeout))
		n, err := conn.Read(buf)
		if err != nil {
			continue // timed out waiting for hello response, retry
		}
		h := parseHeader(buf[:n])
		if h.flags&flagNewSessionID == 0 {
			continue
		}
		sendAck(conn, h.sessionID, h.packetID)
		return h.sessionID, h.packetID, nil
	}
	return 0, 0, fmt.Errorf("no response from switcher after 5 attempts")
}

func sendAck(conn *net.UDPConn, sessionID, packetID uint16) {
	ack := buildPacket(flagAckReply, sessionID, packetID, 0, 0, nil)
	conn.Write(ack)
}

func printTallyIfChanged(state *switcherState, lastPrinted **tallyOutput) {
	live, preview := state.liveAndPreview()
	sort.Slice(live, func(i, j int) bool { return live[i] < live[j] })
	sort.Slice(preview, func(i, j int) bool { return preview[i] < preview[j] })
	if live == nil {
		live = []uint16{}
	}
	if preview == nil {
		preview = []uint16{}
	}
	out := tallyOutput{Live: live, Preview: preview}

	if *lastPrinted != nil && (*lastPrinted).equal(out) {
		return
	}
	*lastPrinted = &out

	b, _ := json.Marshal(out)
	fmt.Println(string(b))
}

func (a tallyOutput) equal(b tallyOutput) bool {
	return uint16SliceEqual(a.Live, b.Live) && uint16SliceEqual(a.Preview, b.Preview)
}

func uint16SliceEqual(a, b []uint16) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
