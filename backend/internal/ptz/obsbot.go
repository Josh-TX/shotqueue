package ptz

import (
	"fmt"
	"log"
	"math"
	"net"
	"strconv"
	"sync"
	"time"
)

// ObsbotClient talks to an OBSBOT Tail 2 directly (not via OBSBOT Center) over OSC/UDP. See
// tail-cameras.md. Position is whole degrees: Pan = yaw, Tilt = pitch. Zoom is ignored (the
// camera can't report it), so it's always 0.
type ObsbotClient struct {
	Host string
	Port string

	mu      sync.Mutex
	pos     Position
	updated chan struct{} // closed (and replaced) whenever a position reply arrives
}

const (
	// The camera sends every reply to this fixed port on our side, so one listener serves all
	// Tail 2s, matched to clients by source IP.
	obsbotReplyPort = 57120

	obsbotMoveSpeed = 45 // 0-90; untested what 0 means, so use a middling value
	obsbotPanLimit  = 159
	obsbotTiltLimit = 62
	obsbotTolerance = 1 // degrees, on pan and tilt

	obsbotReplyTimeout = 1500 * time.Millisecond

	addrConnected = "/OBSBOT/WebCam/General/Connected"
	addrPosResp   = "/OBSBOT/WebCam/General/GetGimbalPosInfoResp"
	addrSetDegree = "/OBSBOT/WebCam/General/SetGimMotorDegree"
)

func NewObsbot(host, port string) *ObsbotClient {
	return &ObsbotClient{Host: host, Port: port, updated: make(chan struct{})}
}

// ---- shared reply listener ----

var obsbotListener struct {
	mu      sync.Mutex
	conn    *net.UDPConn
	clients map[string]*ObsbotClient // source IP -> client
}

// obsbotConn lazily binds the reply port and registers c for its camera IP, returning the socket
// (also used for sending) and the camera's address.
func (c *ObsbotClient) register() (*net.UDPConn, *net.UDPAddr, error) {
	dst, err := net.ResolveUDPAddr("udp", net.JoinHostPort(c.Host, c.Port))
	if err != nil {
		return nil, nil, err
	}
	l := &obsbotListener
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.conn == nil {
		conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: obsbotReplyPort})
		if err != nil {
			return nil, nil, fmt.Errorf("can't listen on UDP %d for OBSBOT replies: %w", obsbotReplyPort, err)
		}
		l.conn = conn
		l.clients = map[string]*ObsbotClient{}
		go obsbotReadLoop(conn)
	}
	l.clients[dst.IP.String()] = c
	return l.conn, dst, nil
}

func obsbotReadLoop(conn *net.UDPConn) {
	buf := make([]byte, 2048)
	for {
		n, from, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("[obsbot] listener stopped: %v", err)
			obsbotListener.mu.Lock()
			obsbotListener.conn = nil
			obsbotListener.mu.Unlock()
			return
		}
		addr, args, err := decodeOSC(buf[:n])
		if err != nil || addr != addrPosResp || len(args) != 2 {
			continue
		}
		obsbotListener.mu.Lock()
		c := obsbotListener.clients[from.IP.String()]
		obsbotListener.mu.Unlock()
		if c == nil {
			continue
		}
		// Tail 2 order is [pitch, yaw] (unverified; flip if pan/tilt look swapped).
		c.mu.Lock()
		c.pos = Position{
			Pan:  int(math.Round(args[1])),
			Tilt: int(math.Round(args[0])),
		}
		close(c.updated)
		c.updated = make(chan struct{})
		c.mu.Unlock()
	}
}

func (c *ObsbotClient) send(addr string, args ...int32) (waitFor <-chan struct{}, err error) {
	conn, dst, err := c.register()
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	ch := c.updated
	c.mu.Unlock()
	_, err = conn.WriteToUDP(encodeOSC(addr, args...), dst)
	return ch, err
}

// ---- Camera ----

func (c *ObsbotClient) GetPosition() (Position, error) {
	if _, err := strconv.Atoi(c.Port); err != nil {
		return Position{}, fmt.Errorf("invalid port %q", c.Port)
	}
	ch, err := c.send(addrConnected, 0)
	if err != nil {
		return Position{}, err
	}
	select {
	case <-ch:
	case <-time.After(obsbotReplyTimeout):
		return Position{}, fmt.Errorf("no position reply from %s:%s", c.Host, c.Port)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pos, nil
}

func (c *ObsbotClient) SetPositionRaw(pos Position) error {
	pan := int32(clamp(pos.Pan, -obsbotPanLimit, obsbotPanLimit))
	tilt := int32(clamp(pos.Tilt, -obsbotTiltLimit, obsbotTiltLimit))
	_, err := c.send(addrSetDegree, obsbotMoveSpeed, pan, tilt)
	return err
}

// CloseEnough ignores zoom, which the Tail 2 can't report.
func (c *ObsbotClient) CloseEnough(a, b Position) bool {
	return abs(a.Pan-b.Pan) <= obsbotTolerance && abs(a.Tilt-b.Tilt) <= obsbotTolerance
}

func (c *ObsbotClient) SupportsFocus() bool        { return false }
func (c *ObsbotClient) GetFocus() (int, error)     { return 0, ErrUnsupported }
func (c *ObsbotClient) SetFocus(int) error         { return ErrUnsupported }
func (c *ObsbotClient) OneTouchFocus() error       { return ErrUnsupported }
func (c *ObsbotClient) IsAutoFocus() (bool, error) { return false, ErrUnsupported }
func (c *ObsbotClient) Snapshot() ([]byte, error)  { return unsupportedJPEG, nil }
