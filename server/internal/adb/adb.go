package adb

import (
	"context"
	"fmt"
	"os/exec"
)

// Defaults for target device.
const (
	DefaultHost = "192.168.1.11"
	// DefaultPort = "41979"
	DefaultPort = "36275"
)

// Key codes (subset used by the app).
type KeyCode int

const (
	// Power toggle: KEYCODE_POWER = 26
	KeycodePower KeyCode = 26

	// Volume controls
	KeycodeVolumeUp   KeyCode = 24
	KeycodeVolumeDown KeyCode = 25

	// Mic mute (note: mutes microphone, not speakers)
	KeycodeMute KeyCode = 91

	// Navigation
	KeycodeHome       KeyCode = 3
	KeycodeBack       KeyCode = 4
	KeycodeDpadUp     KeyCode = 19
	KeycodeDpadDown   KeyCode = 20
	KeycodeDpadLeft   KeyCode = 21
	KeycodeDpadRight  KeyCode = 22
	KeycodeDpadCenter KeyCode = 23

	// Media controls
	KeycodeMediaPlayPause KeyCode = 85
	KeycodeMediaNext      KeyCode = 87
	KeycodeMediaPrevious  KeyCode = 88
	KeycodeMediaStop      KeyCode = 86

	// Menu and Settings
	KeycodeMenu     KeyCode = 82
	KeycodeSettings KeyCode = 176

	// Audio
	KeycodeSpeakerMute KeyCode = 164

	// Input and Misc
	KeycodeInputSource KeyCode = 178
	KeycodeFavourite   KeyCode = 1554
)

// Client wraps ADB target information.
type Client struct {
	Host string
	Port string
}

// NewClient creates a Client with DefaultHost/DefaultPort.
func NewClient() *Client {
	return &Client{
		Host: DefaultHost,
		Port: DefaultPort,
	}
}

// NewWithTarget creates a Client for a specific host/port.
func NewWithTarget(host, port string) *Client {
	return &Client{Host: host, Port: port}
}

func (c *Client) addr() string { return c.Host + ":" + c.Port }

// connect is harmless if already connected.
func (c *Client) connect(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "adb", "connect", c.addr())
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("adb connect failed: %v - %s", err, out)
	}
	return nil
}

// sendKey connects (idempotent) and sends a single key event.
func (c *Client) sendKey(ctx context.Context, code KeyCode) error {
	if err := c.connect(ctx); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "adb", "-s", c.addr(), "shell", "input", "keyevent", fmt.Sprintf("%d", code))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("adb shell failed: %v - %s", err, out)
	}
	return nil
}

// Exposed API methods for app usage.

func (c *Client) PowerToggle(ctx context.Context) error {
	return c.sendKey(ctx, KeycodePower)
}

func (c *Client) VolumeUp(ctx context.Context) error {
	return c.sendKey(ctx, KeycodeVolumeUp)
}

func (c *Client) VolumeDown(ctx context.Context) error {
	return c.sendKey(ctx, KeycodeVolumeDown)
}

func (c *Client) MicMute(ctx context.Context) error {
	return c.sendKey(ctx, KeycodeMute)
}

func (c *Client) Home(ctx context.Context) error {
	return c.sendKey(ctx, KeycodeHome)
}

func (c *Client) Back(ctx context.Context) error {
	return c.sendKey(ctx, KeycodeBack)
}

func (c *Client) MediaPlayPause(ctx context.Context) error {
	return c.sendKey(ctx, KeycodeMediaPlayPause)
}

func (c *Client) NextTrack(ctx context.Context) error {
	return c.sendKey(ctx, KeycodeMediaNext)
}

func (c *Client) PreviousTrack(ctx context.Context) error {
	return c.sendKey(ctx, KeycodeMediaPrevious)
}

func (c *Client) MediaStop(ctx context.Context) error {
	return c.sendKey(ctx, KeycodeMediaStop)
}

func (c *Client) DpadUp(ctx context.Context) error {
	return c.sendKey(ctx, KeycodeDpadUp)
}

func (c *Client) DpadDown(ctx context.Context) error {
	return c.sendKey(ctx, KeycodeDpadDown)
}

func (c *Client) DpadLeft(ctx context.Context) error {
	return c.sendKey(ctx, KeycodeDpadLeft)
}

func (c *Client) DpadRight(ctx context.Context) error {
	return c.sendKey(ctx, KeycodeDpadRight)
}

func (c *Client) DpadCenter(ctx context.Context) error {
	return c.sendKey(ctx, KeycodeDpadCenter)
}

func (c *Client) Menu(ctx context.Context) error {
	return c.sendKey(ctx, KeycodeMenu)
}

func (c *Client) Settings(ctx context.Context) error {
	return c.sendKey(ctx, KeycodeSettings)
}

func (c *Client) SpeakerMute(ctx context.Context) error {
	return c.sendKey(ctx, KeycodeSpeakerMute)
}

func (c *Client) InputSource(ctx context.Context) error {
	return c.sendKey(ctx, KeycodeInputSource)
}

func (c *Client) Favourite(ctx context.Context) error {
	return c.sendKey(ctx, KeycodeFavourite)
}

// Optional: generic method if the app needs to send arbitrary keycodes.
func (c *Client) SendKey(ctx context.Context, code KeyCode) error {
	return c.sendKey(ctx, code)
}

// ADB Shell Input Keyevent Reference
//
// Usage:
//   adb shell
//   input keyevent <keycode>
//
// To discover keycodes:
//   logcat | grep -i keyevent
//
// ┌──────────────────────────────┬─────────┬────────────────────────────────────────────────────────┐
// │ Function                     │ Keycode │ Notes / Limitations                                    │
// ├──────────────────────────────┼─────────┼────────────────────────────────────────────────────────┤
// │ Power Toggle                 │ 26      │ Toggles standby/on if reachable; cannot turn on from  │
// │                              │         │ fully off.                                             │
// │ Volume Up                    │ 24      │ Works incrementally.                                   │
// │ Volume Down                  │ 25      │ Works incrementally.                                   │
// │ Mic Mute (voice input)       │ 91      │ Does NOT mute speakers, only mutes microphone.         │
// │ Home / Launcher              │ 3       │ Opens Google TV launcher.                              │
// │ Back / Exit                  │ 4       │ Works in menus and apps.                               │
// │ Media Play / Pause           │ 85      │ Works in media apps.                                   │
// │ Media Next Track             │ 87      │ Works in media apps.                                   │
// │ Media Previous Track         │ 88      │ Works in media apps.                                   │
// │ Media Stop                   │ 86      │ Works in media apps.                                   │
// │ Settings                     │ 176     │ Opens settings menu.                                   │
// │ Input Source Menu            │ 178     │ Opens input/select menu.                               │
// │ DPAD Up                      │ 19      │ Navigation.                                            │
// │ DPAD Down                    │ 20      │ Navigation.                                            │
// │ DPAD Left                    │ 21      │ Navigation.                                            │
// │ DPAD Right                   │ 22      │ Navigation.                                            │
// │ DPAD Center / OK             │ 23      │ Select.                                                │
// │ Menu / Options               │ 82      │ Opens options menu.                                    │
// │ Mute (Speaker mute)          │ 164     │ NOT mic mute.                                          │
// │ Favourite Button             │ 1554    │ Found despite Gippty leading me astray.                │
// │ Weird Power Off			  │ 177     │ weird shutdown, adb disconnecs (DO NOT USE)			 │
// └──────────────────────────────┴─────────┴────────────────────────────────────────────────────────┘
