// Package atem is a read-only ATEM tally client, ported from go-atem-listener: connects to a real
// ATEM (or mock-atem) over UDP:9910 and reports which sources are live/preview. No control.
package atem

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"
)

const (
	atemPort           = 9910
	handshakeTimeout   = 2 * time.Second
	establishedTimeout = 5 * time.Second // no packets received for this long => connection considered dead
)

// Listener connects to one ATEM's tally feed and invokes OnChange whenever live/preview changes.
type Listener struct {
	Host     string
	OnChange func(TallyState)
}

// Run connects and reconnects with backoff until ctx is cancelled.
func (l *Listener) Run(ctx context.Context) {
	backoff := time.Second
	for {
		err := l.connectAndRun(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Printf("[atem] disconnected from %s: %v", l.Host, err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 8*time.Second {
			backoff *= 2
		}
	}
}

func (l *Listener) connectAndRun(ctx context.Context) error {
	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", l.Host, atemPort))
	if err != nil {
		return fmt.Errorf("invalid ATEM host %q: %w", l.Host, err)
	}
	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	sessionID, lastReceivedPacketID, err := handshake(conn)
	if err != nil {
		return fmt.Errorf("handshake: %w", err)
	}
	log.Printf("[atem] connected to %s (session %d)", l.Host, sessionID)

	state := newSwitcherState()

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
						if state.applyCommand(cmd) && l.OnChange != nil {
							l.OnChange(state.snapshot())
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
