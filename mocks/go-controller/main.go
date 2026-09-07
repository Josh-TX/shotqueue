package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"go-controller/internal/auth"
)

// publicDir resolves relative to this source file rather than the working directory, so `go run
// ./go-controller` works the same from anywhere.
func publicDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "public")
}

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

func reportAuth(event auth.Event) {
	scheme := ""
	if event.Scheme != "" {
		scheme = fmt.Sprintf(" (%s)", event.Scheme)
	}
	log.Printf("[auth] %s%s", event.Type, scheme)
}

// authedGet adapts net/http to what auth.RequestWithAuth expects from send: a 401/403 must come
// back as *auth.StatusError.
func authedGet(client *http.Client, session *auth.Session, url, uri string) (*http.Response, error) {
	return auth.RequestWithAuth(func(headers map[string]string) (*http.Response, error) {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			resp.Body.Close()
			return nil, &auth.StatusError{StatusCode: resp.StatusCode, Header: resp.Header}
		}
		return resp, nil
	}, session, http.MethodGet, uri, reportAuth)
}

func parsePort(argv []string) int {
	for _, arg := range argv {
		if strings.HasPrefix(arg, "--port=") {
			if port, err := strconv.Atoi(strings.TrimPrefix(arg, "--port=")); err == nil {
				return port
			}
		}
	}
	if p := os.Getenv("PORT"); p != "" {
		if port, err := strconv.Atoi(p); err == nil {
			return port
		}
	}
	return 8080
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

type controller struct {
	client  *http.Client
	session *auth.Session
}

func (c *controller) camGet(host, port, cmd string) (string, error) {
	uri := fmt.Sprintf("/cgi-bin/aw_ptz?cmd=%%23%s&res=1", cmd)
	url := fmt.Sprintf("http://%s:%s%s", host, port, uri)
	resp, err := authedGet(c.client, c.session, url, uri)
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

type position struct {
	Pan  int `json:"pan"`
	Tilt int `json:"tilt"`
	Zoom int `json:"zoom"`
}

func (c *controller) getPosition(host, port string) (position, error) {
	body, err := c.camGet(host, port, "PTV")
	if err != nil {
		return position{}, err
	}
	if len(body) < 14 {
		return position{}, fmt.Errorf("unexpected response: %s", body)
	}
	pan, err := strconv.ParseInt(body[3:7], 16, 32)
	if err != nil {
		return position{}, err
	}
	tilt, err := strconv.ParseInt(body[7:11], 16, 32)
	if err != nil {
		return position{}, err
	}
	zoom, err := strconv.ParseInt(body[11:14], 16, 32)
	if err != nil {
		return position{}, err
	}
	return position{Pan: int(pan), Tilt: int(tilt), Zoom: int(zoom)}, nil
}

func (c *controller) setPosition(host, port string, panPct, tiltPct, zoomPct float64) (position, error) {
	pan := clamp(pctToValue(panPct, panMin, panMax), panMin, panMax)
	tilt := clamp(pctToValue(tiltPct, tiltMin, tiltMax), tiltMin, tiltMax)
	zoom := clamp(pctToValue(zoomPct, zoomMin, zoomMax), zoomMin, zoomMax)

	if _, err := c.camGet(host, port, fmt.Sprintf("APC%s%s", toHex(pan, 4), toHex(tilt, 4))); err != nil {
		return position{}, err
	}
	if _, err := c.camGet(host, port, fmt.Sprintf("AXZ%s", toHex(zoom, 3))); err != nil {
		return position{}, err
	}
	return position{Pan: pan, Tilt: tilt, Zoom: zoom}, nil
}

func writeError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func (c *controller) handlePosition(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		host := r.URL.Query().Get("host")
		port := r.URL.Query().Get("port")
		pos, err := c.getPosition(host, port)
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pos)

	case http.MethodPost:
		var body struct {
			Host    string  `json:"host"`
			Port    string  `json:"port"`
			PanPct  float64 `json:"panPct"`
			TiltPct float64 `json:"tiltPct"`
			ZoomPct float64 `json:"zoomPct"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		pos, err := c.setPosition(body.Host, body.Port, body.PanPct, body.TiltPct, body.ZoomPct)
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pos)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (c *controller) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	host := r.URL.Query().Get("host")
	port := r.URL.Query().Get("port")
	uri := fmt.Sprintf("/cgi-bin/view.cgi?action=snapshot&n=%d", time.Now().UnixMilli())
	url := fmt.Sprintf("http://%s:%s%s", host, port, uri)

	resp, err := authedGet(c.client, c.session, url, uri)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "image/jpeg")
	io.Copy(w, resp.Body)
}

func main() {
	port := parsePort(os.Args[1:])

	// mock-ue150's only account. Harmless if the camera's auth is off; auth.RequestWithAuth only
	// sends it once the camera actually challenges for it.
	session := auth.NewSession("admin", "12345")

	c := &controller{client: &http.Client{}, session: session}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/position", c.handlePosition)
	mux.HandleFunc("/api/snapshot", c.handleSnapshot)
	mux.Handle("/", http.FileServer(http.Dir(publicDir())))

	log.Printf("poc-controller listening on http://localhost:%d", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), mux))
}
