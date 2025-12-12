package device

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"
)

type RelayState struct {
	Label string `json:"label"`
	State bool   `json:"state"`
}

type DeviceState struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

type Manager struct {
	relays io.ReadWriteCloser
	buzzer io.ReadWriteCloser

	relayStates  [8]int
	relayLabels  [8]string
	relayStatesM sync.RWMutex

	deviceM sync.RWMutex

	dialers      map[string]func() (io.ReadWriteCloser, error)
	reconnectM   sync.Mutex
	reconnecting map[string]bool
}

func NewManager() *Manager {
	return &Manager{
		dialers:      make(map[string]func() (io.ReadWriteCloser, error)),
		reconnecting: make(map[string]bool),
	}
}

func (m *Manager) SetDialer(name string, d func() (io.ReadWriteCloser, error)) {
	m.reconnectM.Lock()
	defer m.reconnectM.Unlock()
	m.dialers[name] = d
}

func (m *Manager) SetDevice(name string, d io.ReadWriteCloser) {
	m.deviceM.Lock()
	defer m.deviceM.Unlock()
	switch name {
	case "relays":
		m.relays = d
	case "buzzer":
		m.buzzer = d
	}
}

func (m *Manager) GetDevice(name string) io.ReadWriteCloser {
	m.deviceM.RLock()
	defer m.deviceM.RUnlock()
	switch name {
	case "relays":
		return m.relays
	case "buzzer":
		return m.buzzer
	}
	return nil
}

func (m *Manager) StartReader(name string) {
	if d := m.GetDevice(name); d != nil {
		go m.readFromDevice(name, d)
	}
}

func (m *Manager) readFromDevice(deviceName string, dev io.ReadWriteCloser) {
	if dev == nil {
		return
	}
	reader := bufio.NewReader(dev)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				time.Sleep(200 * time.Millisecond)
				continue
			}
			slog.Error("device read error", "device", deviceName, "err", err)
			_ = dev.Close()
			m.SetDevice(deviceName, nil)
			dial := func() (io.ReadWriteCloser, error) { return nil, fmt.Errorf("no dialer") }
			m.reconnectM.Lock()
			if d, ok := m.dialers[deviceName]; ok {
				dial = d
			}
			m.reconnectM.Unlock()
			m.startReconnectIfNeeded(deviceName, dial)
			return
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "HB:") {
			// NO HEARTBEAT
			// slog.Info("heartbeat", "device", deviceName, "HB", line[3:])
			continue
		}
		slog.Info("read", "device", deviceName, "line", line)

		if strings.HasPrefix(line, "RELAYS:") {
			hexStr := strings.TrimSpace(strings.TrimPrefix(line, "RELAYS:"))
			if strings.HasPrefix(hexStr, "0x") || strings.HasPrefix(hexStr, "0X") {
				hexStr = hexStr[2:]
			}
			val, perr := strconv.ParseUint(hexStr, 16, 8)
			if perr != nil {
				slog.Error("invalid RELAYS line", "device", deviceName, "value", hexStr, "err", perr)
				continue
			}
			b := byte(val)

			m.relayStatesM.Lock()
			for i := 0; i < len(m.relayStates); i++ {
				m.relayStates[i] = int((b >> i) & 1)
			}
			m.relayStatesM.Unlock()

			slog.Info("relay states updated", "device", deviceName, "bitmask", fmt.Sprintf("%08b", b))
			continue
		}
	}
}

func (m *Manager) startReconnectIfNeeded(name string, dial func() (io.ReadWriteCloser, error)) {
	m.reconnectM.Lock()
	if m.reconnecting[name] {
		m.reconnectM.Unlock()
		return
	}
	m.reconnecting[name] = true
	m.reconnectM.Unlock()

	go func() {
		defer func() {
			m.reconnectM.Lock()
			m.reconnecting[name] = false
			m.reconnectM.Unlock()
		}()
		delay := 1 * time.Second
		maxDelay := 30 * time.Second
		attempt := 0
		for {
			attempt++
			slog.Warn("attempting reconnect", "device", name, "attempt", attempt)
			slog.Info("dialing", "device", name, "attempt", attempt)
			start := time.Now()
			dev, err := dial()
			dur := time.Since(start)
			if err != nil {
				slog.Error("reconnect failed", "device", name, "attempt", attempt, "err", err, "retry_in", delay, "dur", dur)
				time.Sleep(delay)
				if delay < maxDelay {
					delay *= 2
					if delay > maxDelay {
						delay = maxDelay
					}
				}
				continue
			}
			m.SetDevice(name, dev)
			slog.Info("device connected", "device", name, "attempt", attempt, "dur", dur)
			go m.readFromDevice(name, dev)
			return
		}
	}()
}

func (m *Manager) RelayStates() []RelayState {
	m.relayStatesM.RLock()
	defer m.relayStatesM.RUnlock()
	states := make([]RelayState, len(m.relayStates))
	for i := 0; i < len(m.relayStates); i++ {
		states[i] = RelayState{
			Label: m.relayLabels[i],
			State: m.relayStates[i] != 0,
		}
	}
	return states
}

func (m *Manager) UpdateLabel(index int, label string) {
	m.relayStatesM.Lock()
	defer m.relayStatesM.Unlock()
	if index >= 0 && index < len(m.relayLabels) {
		m.relayLabels[index] = label
	}
}

func (m *Manager) SetLabels(labels [8]string) {
	m.relayStatesM.Lock()
	defer m.relayStatesM.Unlock()
	m.relayLabels = labels
}

func (m *Manager) ToggleRelay(id string) error {
	if len(id) != 1 || id[0] < '1' || id[0] > '8' {
		return fmt.Errorf("invalid relay id")
	}
	d := m.GetDevice("relays")
	if d == nil {
		return fmt.Errorf("relays not connected")
	}
	if _, err := d.Write([]byte(id)); err != nil {
		_ = d.Close()
		m.SetDevice("relays", nil)
		slog.Warn("device write failed; scheduling reconnect", "device", "relays", "err", err)
		m.reconnectM.Lock()
		if dial, ok := m.dialers["relays"]; ok {
			m.reconnectM.Unlock()
			m.startReconnectIfNeeded("relays", dial)
		} else {
			m.reconnectM.Unlock()
		}
		return fmt.Errorf("write failed: %w", err)
	}
	return nil
}

func (m *Manager) BuzzDoor() error {
	d := m.GetDevice("buzzer")
	if d == nil {
		return fmt.Errorf("buzzer not connected")
	}
	if _, err := d.Write([]byte("1")); err != nil {
		_ = d.Close()
		m.SetDevice("buzzer", nil)
		slog.Warn("device write failed; scheduling reconnect", "device", "buzzer", "err", err)
		m.reconnectM.Lock()
		if dial, ok := m.dialers["buzzer"]; ok {
			m.reconnectM.Unlock()
			m.startReconnectIfNeeded("buzzer", dial)
		} else {
			m.reconnectM.Unlock()
		}
		return fmt.Errorf("write failed: %w", err)
	}
	return nil
}
