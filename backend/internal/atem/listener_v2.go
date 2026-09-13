// Package atem is a read-only ATEM tally client, ported from mocks/go-atem-listener-v2: connects
// to a real ATEM (or mock-atem) over UDP:9910 and reports which sources are live/preview. No
// control.
//
// An earlier version of this listener sent a client-initiated RETRANSMIT_REQUEST packet whenever
// it saw a gap in packet IDs. That worked fine against a mock/localhost, where packets never
// arrive out of order, but real ATEM hardware treats that request as malformed and simply goes
// silent to the client - one clean handshake, one correct initial state read, then nothing. This
// version never builds or sends that packet: on a gap it just re-ACKs the last known-good packet
// ID (if the incoming packet is already covered by that ACK) and otherwise does nothing, relying
// entirely on the switcher's own send-side retry timer to resend what was missed.
package atem

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

const (
	handshakeRetryInterval = 2 * time.Second
	establishedTimeout     = 10 * time.Second // no packets received for this long => connection considered dead
)

// Listener connects to one ATEM's tally feed and invokes OnChange whenever live/preview changes.
type Listener struct {
	Host         string
	OnChange     func(TallyState)
	OnConnect    func()
	OnDisconnect func()

	conn *net.UDPConn

	mu                   sync.Mutex
	sessionID            uint16
	lastReceivedPacketID uint16
	established          bool
	state                *switcherState
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

func (l *Listener) connectAndRun(ctx context.Context) (err error) {
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

	l.conn = conn
	l.mu.Lock()
	l.established = false
	l.state = newSwitcherState()
	l.mu.Unlock()

	connected := false
	defer func() {
		if connected && l.OnDisconnect != nil {
			l.OnDisconnect()
		}
	}()

	l.send(commandConnectHello)

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

		n, err := conn.Read(buf)
		if err != nil {
			if established {
				return fmt.Errorf("no data from switcher for %s", establishedTimeout)
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			l.send(commandConnectHello)
			continue
		}

		packet := make([]byte, n)
		copy(packet, buf[:n])
		l.onMessage(packet)

		if !connected {
			l.mu.Lock()
			nowEstablished := l.established
			l.mu.Unlock()
			if nowEstablished {
				connected = true
				log.Printf("[atem] connected to %s", l.Host)
				if l.OnConnect != nil {
					l.OnConnect()
				}
			}
		}
	}
}

func (l *Listener) send(buf []byte) {
	l.conn.Write(buf)
}

func (l *Listener) sendAck(packetID uint16) {
	l.mu.Lock()
	sessionID := l.sessionID
	l.mu.Unlock()
	l.send(buildPacket(flagAckReply, sessionID, packetID, 0, 0, nil))
}

// onMessage handles a single packet from the switcher. See the package comment for why it never
// sends a RETRANSMIT_REQUEST on a gap.
func (l *Listener) onMessage(packet []byte) {
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
		l.sendAck(remotePacketID)
		return
	}

	l.mu.Lock()
	established := l.established
	l.mu.Unlock()
	if !established {
		return
	}

	if flags&flagAckRequest == 0 {
		return
	}

	l.mu.Lock()
	expected := (l.lastReceivedPacketID + 1) % maxPacketID
	l.mu.Unlock()

	if remotePacketID == expected {
		l.mu.Lock()
		l.lastReceivedPacketID = remotePacketID
		state := l.state
		l.mu.Unlock()
		l.sendAck(remotePacketID)
		if length > 12 {
			for _, cmd := range parseCommands(packet[12:]) {
				if state.applyCommand(cmd) && l.OnChange != nil {
					l.OnChange(state.snapshot())
				}
			}
		}
		return
	}

	l.mu.Lock()
	lastReceived := l.lastReceivedPacketID
	l.mu.Unlock()
	if isPacketCoveredByAck(lastReceived, remotePacketID) {
		l.sendAck(lastReceived)
	}
}
