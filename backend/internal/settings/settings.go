// Package settings loads and saves shotqueue's settings — the ATEM address and the camera
// connection roster (host/port/username/password, keyed by CameraNum) — to a JSON file in the
// user's config directory. Presets and groups are persisted separately via internal/config (the
// "latest" config), not here.
package settings

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Atem struct {
	Host string `json:"host"`
}

// CameraSettings holds one camera's connection info. Password is plaintext in memory; it's only
// ever encrypted at the moment it's written to disk (see saveLocked/Load), with a hardcoded key —
// this is meant to avoid a plaintext password sitting in the settings file, not to be
// cryptographically secure. Password is never sent to the frontend and can only be set when a
// camera is first added (see api.handleSettingsCameras).
type CameraSettings struct {
	CameraNum int    `json:"cameraNum"`
	Host      string `json:"host"`
	Port      string `json:"port"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Hidden    bool   `json:"hidden"`
}

type Settings struct {
	Atem    Atem             `json:"atem"`
	Cameras []CameraSettings `json:"cameras"`
}

type Store struct {
	mu   sync.Mutex
	path string
	cfg  Settings
}

func dirPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "shotqueue"), nil
}

// fileCameraSettings mirrors CameraSettings but with Password encrypted+base64 for on-disk storage.
type fileCameraSettings struct {
	CameraNum int    `json:"cameraNum"`
	Host      string `json:"host"`
	Port      string `json:"port"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Hidden    bool   `json:"hidden"`
}

type fileFormat struct {
	Atem    Atem                 `json:"atem"`
	Cameras []fileCameraSettings `json:"cameras"`
}

// Load reads settings.json from the user config directory, creating an empty default if it doesn't
// exist yet.
func Load() (*Store, error) {
	dir, err := dirPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "settings.json")

	s := &Store{path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, s.saveLocked()
		}
		return nil, err
	}
	var f fileFormat
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	s.cfg.Atem = f.Atem
	s.cfg.Cameras = make([]CameraSettings, len(f.Cameras))
	for i, fc := range f.Cameras {
		password, err := decryptPassword(fc.Password)
		if err != nil {
			return nil, fmt.Errorf("decrypting password for camera %d: %w", fc.CameraNum, err)
		}
		s.cfg.Cameras[i] = CameraSettings{
			CameraNum: fc.CameraNum,
			Host:      fc.Host,
			Port:      fc.Port,
			Username:  fc.Username,
			Password:  password,
			Hidden:    fc.Hidden,
		}
	}
	return s, nil
}

func (s *Store) saveLocked() error {
	f := fileFormat{Atem: s.cfg.Atem, Cameras: make([]fileCameraSettings, len(s.cfg.Cameras))}
	for i, c := range s.cfg.Cameras {
		password, err := encryptPassword(c.Password)
		if err != nil {
			return fmt.Errorf("encrypting password for camera %d: %w", c.CameraNum, err)
		}
		f.Cameras[i] = fileCameraSettings{
			CameraNum: c.CameraNum,
			Host:      c.Host,
			Port:      c.Port,
			Username:  c.Username,
			Password:  password,
			Hidden:    c.Hidden,
		}
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

// encKey is hardcoded on purpose: this encryption exists only to avoid storing passwords in plain
// text on disk, not to withstand anyone with access to the binary/source.
var encKey = sha256.Sum256([]byte("shotqueue-static-settings-key"))

func encryptPassword(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	block, err := aes.NewCipher(encKey[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decryptPassword(enc string) (string, error) {
	if enc == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(encKey[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (s *Store) Get() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.cfg
	out.Cameras = make([]CameraSettings, len(s.cfg.Cameras))
	copy(out.Cameras, s.cfg.Cameras)
	return out
}

func (s *Store) SetAtem(atem Atem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg.Atem = atem
	return s.saveLocked()
}

// Cameras returns every camera's connection settings, including plaintext passwords — for internal
// backend use only (building ptz.Client). Never expose this directly to the frontend.
func (s *Store) Cameras() []CameraSettings {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]CameraSettings, len(s.cfg.Cameras))
	copy(out, s.cfg.Cameras)
	return out
}

func (s *Store) findCameraLocked(num int) int {
	for i, c := range s.cfg.Cameras {
		if c.CameraNum == num {
			return i
		}
	}
	return -1
}

// CameraByNum returns one camera's settings (including plaintext password), for internal use.
func (s *Store) CameraByNum(num int) (CameraSettings, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.findCameraLocked(num)
	if idx == -1 {
		return CameraSettings{}, false
	}
	return s.cfg.Cameras[idx], true
}

// AddCamera registers a new camera. cs.CameraNum must not already be in use.
func (s *Store) AddCamera(cs CameraSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.findCameraLocked(cs.CameraNum) != -1 {
		return fmt.Errorf("camera number %d already exists", cs.CameraNum)
	}
	s.cfg.Cameras = append(s.cfg.Cameras, cs)
	return s.saveLocked()
}

// UpdateCamera edits an existing camera's number/host/port/username/hidden (password is
// intentionally not editable here — see CameraSettings doc comment). If newNum differs from
// currentNum, it must not already be in use by another camera.
func (s *Store) UpdateCamera(currentNum, newNum int, host, port, username string, hidden bool) (CameraSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.findCameraLocked(currentNum)
	if idx == -1 {
		return CameraSettings{}, errors.New("camera not found")
	}
	if newNum != currentNum {
		if s.findCameraLocked(newNum) != -1 {
			return CameraSettings{}, fmt.Errorf("camera number %d already exists", newNum)
		}
		s.cfg.Cameras[idx].CameraNum = newNum
	}
	s.cfg.Cameras[idx].Host = host
	s.cfg.Cameras[idx].Port = port
	s.cfg.Cameras[idx].Username = username
	s.cfg.Cameras[idx].Hidden = hidden
	if err := s.saveLocked(); err != nil {
		return CameraSettings{}, err
	}
	return s.cfg.Cameras[idx], nil
}

// SetHidden is a narrow helper used when auto-unhiding a camera referenced by a config being
// loaded (see state.BuildLoadSeeds).
func (s *Store) SetHidden(num int, hidden bool) (CameraSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.findCameraLocked(num)
	if idx == -1 {
		return CameraSettings{}, errors.New("camera not found")
	}
	s.cfg.Cameras[idx].Hidden = hidden
	if err := s.saveLocked(); err != nil {
		return CameraSettings{}, err
	}
	return s.cfg.Cameras[idx], nil
}

func (s *Store) DeleteCamera(num int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.findCameraLocked(num)
	if idx == -1 {
		return errors.New("camera not found")
	}
	s.cfg.Cameras = append(s.cfg.Cameras[:idx], s.cfg.Cameras[idx+1:]...)
	return s.saveLocked()
}
