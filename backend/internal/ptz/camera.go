package ptz

import (
	_ "embed"
	"errors"
)

// Camera types, as stored in settings.
const (
	TypePanasonic = "panasonic"
	TypeObsbot    = "obsbot"
)

// ErrUnsupported is returned by operations a camera type can't do (e.g. focus on the Tail 2).
var ErrUnsupported = errors.New("not supported by this camera")

// Camera is what the rest of the app needs from a PTZ camera. Position units are native to each
// camera type (Panasonic raw hex values, OBSBOT whole degrees), so only the same type's
// CloseEnough should compare two Positions.
type Camera interface {
	GetPosition() (Position, error)
	// SetPositionRaw moves to an exact position as returned by GetPosition.
	SetPositionRaw(pos Position) error
	// CloseEnough reports whether a live reading counts as being at target.
	CloseEnough(reading, target Position) bool

	// SupportsFocus is false when the focus methods below just return ErrUnsupported.
	SupportsFocus() bool
	GetFocus() (int, error)
	SetFocus(focus int) error
	OneTouchFocus() error
	IsAutoFocus() (bool, error)

	// Snapshot returns one JPEG frame (or a placeholder if the camera can't provide one).
	Snapshot() ([]byte, error)
}

// ValidType reports whether t is a known camera type.
func ValidType(t string) bool { return t == TypePanasonic || t == TypeObsbot }

// NewCamera builds a client for the given type. An empty type means Panasonic, since settings
// saved before camera types existed have none.
func NewCamera(cameraType, host, port, username, password string) Camera {
	if cameraType == TypeObsbot {
		return NewObsbot(host, port)
	}
	return NewPanasonic(host, port, username, password)
}

//go:embed unsupported.jpg
var unsupportedJPEG []byte
