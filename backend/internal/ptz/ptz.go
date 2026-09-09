// Package ptz talks to an AW-UE150 (or mock-ue150) over its HTTP CGI interface, ported from
// mock-controller. One Client is bound to a single camera's host:port.
package ptz

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"shotqueue-backend/internal/ptz/auth"
)

// Must match mock-ue150's ranges (tilt intentionally reuses the pan range there, so it does here
// too).
const (
	panMin  = 0x2d09
	panMax  = 0xd2f5
	tiltMin = panMin
	tiltMax = panMax
	zoomMin = 0x555
	zoomMax = 0xfff
)

type Position struct {
	Pan  int `json:"pan"`
	Tilt int `json:"tilt"`
	Zoom int `json:"zoom"`
}

type Client struct {
	Host    string
	Port    string
	client  *http.Client
	session *auth.Session
}

// mock-ue150's only account. Harmless if the camera's auth is off; auth.RequestWithAuth only
// sends it once the camera actually challenges for it.
func New(host, port string) *Client {
	return &Client{
		Host:    host,
		Port:    port,
		client:  &http.Client{Timeout: 5 * time.Second},
		session: auth.NewSession("admin", "wrongpassword"),
	}
}

func reportAuth(event auth.Event) {
	// intentionally silent by default; callers can add logging by wrapping Client if needed
	_ = event
}

func (c *Client) authedGet(url, uri string) (*http.Response, error) {
	return auth.RequestWithAuth(func(headers map[string]string) (*http.Response, error) {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := c.client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode >= 400 {
			resp.Body.Close()
			return nil, &auth.StatusError{StatusCode: resp.StatusCode, Header: resp.Header}
		}
		return resp, nil
	}, c.session, http.MethodGet, uri, reportAuth)
}

func toHex(value int, width int) string {
	return strings.ToUpper(fmt.Sprintf("%0*x", width, value))
}

func pctToValue(pct float64, min, max int) int {
	return int(math.Round(float64(min) + float64(max-min)*(pct/100)))
}

func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func (c *Client) camGet(cmd string) (string, error) {
	uri := fmt.Sprintf("/cgi-bin/aw_ptz?cmd=%%23%s&res=1", cmd)
	url := fmt.Sprintf("http://%s:%s%s", c.Host, c.Port, uri)
	resp, err := c.authedGet(url, uri)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(string(body), "eR") {
		return "", fmt.Errorf("camera error: %s", body)
	}
	return string(body), nil
}

func (c *Client) GetPosition() (Position, error) {
	body, err := c.camGet("PTV")
	if err != nil {
		return Position{}, err
	}
	if len(body) < 14 {
		return Position{}, fmt.Errorf("unexpected response: %s", body)
	}
	pan, err := strconv.ParseInt(body[3:7], 16, 32)
	if err != nil {
		return Position{}, err
	}
	tilt, err := strconv.ParseInt(body[7:11], 16, 32)
	if err != nil {
		return Position{}, err
	}
	zoom, err := strconv.ParseInt(body[11:14], 16, 32)
	if err != nil {
		return Position{}, err
	}
	return Position{Pan: int(pan), Tilt: int(tilt), Zoom: int(zoom)}, nil
}

func (c *Client) SetPosition(panPct, tiltPct, zoomPct float64) (Position, error) {
	pan := clamp(pctToValue(panPct, panMin, panMax), panMin, panMax)
	tilt := clamp(pctToValue(tiltPct, tiltMin, tiltMax), tiltMin, tiltMax)
	zoom := clamp(pctToValue(zoomPct, zoomMin, zoomMax), zoomMin, zoomMax)

	if _, err := c.camGet(fmt.Sprintf("APC%s%s", toHex(pan, 4), toHex(tilt, 4))); err != nil {
		return Position{}, err
	}
	if _, err := c.camGet(fmt.Sprintf("AXZ%s", toHex(zoom, 3))); err != nil {
		return Position{}, err
	}
	return Position{Pan: pan, Tilt: tilt, Zoom: zoom}, nil
}

// SetPositionRaw sends an exact pan/tilt/zoom (as returned by GetPosition) rather than converting
// from a percentage, so restoring a captured preset doesn't lose precision through pct round-trips.
func (c *Client) SetPositionRaw(pos Position) error {
	if _, err := c.camGet(fmt.Sprintf("APC%s%s", toHex(pos.Pan, 4), toHex(pos.Tilt, 4))); err != nil {
		return err
	}
	if _, err := c.camGet(fmt.Sprintf("AXZ%s", toHex(pos.Zoom, 3))); err != nil {
		return err
	}
	return nil
}

// Snapshot fetches one JPEG frame from the camera.
func (c *Client) Snapshot() ([]byte, error) {
	uri := fmt.Sprintf("/cgi-bin/view.cgi?action=snapshot&n=%d", time.Now().UnixMilli())
	url := fmt.Sprintf("http://%s:%s%s", c.Host, c.Port, uri)
	resp, err := c.authedGet(url, uri)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
