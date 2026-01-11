package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"relaypanel/internal/adb"
	"relaypanel/internal/db"
	"relaypanel/internal/device"
	"relaypanel/internal/logging"
	"relaypanel/internal/router"
	"relaypanel/internal/telnet"

	"github.com/tarm/serial"
)

const (
	SerialBaudArduinoUNO = 9600
	SerialBaudESP32      = 115200
	SerialPortUNO        = "COM5"
	SerialPortESP32      = "COM6"

	RelaysESP32Host = "esp32-1.local"
	BuzzerESP32Host = "esp32-2.local"

	DefaultSerialPort = SerialPortUNO
	DefaultSerialBaud = SerialBaudArduinoUNO

	StatusInterval = 15 * time.Second
)

func dialMultiTelnet(mgr *device.Manager) error {
	mgr.SetDialer("relays", func() (io.ReadWriteCloser, error) { return telnet.DialTelnet(RelaysESP32Host) })
	mgr.SetDialer("buzzer", func() (io.ReadWriteCloser, error) { return telnet.DialTelnet(BuzzerESP32Host) })

	type dialResult struct {
		name string
		conn io.ReadWriteCloser
		err  error
	}
	dialResultChan := make(chan dialResult, 2)

	slog.Info("dialing relays", "host", RelaysESP32Host)
	go func() {
		c, err := telnet.DialTelnet(RelaysESP32Host)
		dialResultChan <- dialResult{name: "relays", conn: c, err: err}
	}()

	slog.Info("dialing buzzer", "host", BuzzerESP32Host)
	go func() {
		c, err := telnet.DialTelnet(BuzzerESP32Host)
		dialResultChan <- dialResult{name: "buzzer", conn: c, err: err}
	}()

	var relays, buzzer io.ReadWriteCloser
	for i := 0; i < 2; i++ {
		r := <-dialResultChan
		if r.err != nil {
			if relays != nil {
				_ = relays.Close()
			}
			if buzzer != nil {
				_ = buzzer.Close()
			}
			return fmt.Errorf("failed to connect %s via telnet: %w", r.name, r.err)
		}
		if r.name == "relays" {
			relays = r.conn
		} else {
			buzzer = r.conn
		}
	}

	mgr.SetDevice("relays", relays)
	slog.Info("connected to relays", "host", RelaysESP32Host)
	mgr.SetDevice("buzzer", buzzer)
	slog.Info("connected to buzzer", "host", BuzzerESP32Host)
	return nil
}

func Run() error {
	logging.Setup()

	serialFlag := flag.String("serial", DefaultSerialPort, "serial COM port")
	telnetFlag := flag.String("telnet", "", "telnet address host:port")
	baudFlag := flag.Int("baud", DefaultSerialBaud, "serial baud rate")
	multiFlag := flag.Bool("multi", false, "connect to both relays and buzzer ESP32s (no args)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [--serial COM] [--telnet host:port] [--baud BAUD] [--multi]\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  # Serial mode (default serial COM5, 9600 baud):")
		fmt.Fprintln(os.Stderr, "  ", os.Args[0], "--serial=COM5 --baud=9600")
		fmt.Fprintln(os.Stderr, "  # Telnet mode (use telnet host:port):")
		fmt.Fprintln(os.Stderr, "  ", os.Args[0], "--telnet=192.168.1.50:23")
		fmt.Fprintln(os.Stderr, "  # Multi telnet mode (connect to relays and buzzer ESPs):")
		fmt.Fprintln(os.Stderr, "  ", os.Args[0], "--multi")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Options:")
		flag.PrintDefaults()
	}

	if len(os.Args) == 1 {
		flag.Usage()
		return nil
	}
	if err := flag.CommandLine.Parse(os.Args[1:]); err != nil {
		flag.Usage()
		return err
	}

	db.Connect(context.Background())

	deviceManager := device.NewManager()

	// Load labels from DB
	var labels [8]string
	if rels := db.ListRelays(context.Background()); rels != nil {
		for _, r := range *rels {
			idx := int(r.RelayIndex) - 1
			if idx >= 0 && idx < len(labels) {
				labels[idx] = r.Label
			}
		}
	}
	deviceManager.SetLabels(labels)
	slog.Info("loaded relay labels from DB")

	modeStr := "serial"

	if *multiFlag {
		err := dialMultiTelnet(deviceManager)
		if err != nil {
			return err
		}
		modeStr = "multi"
	} else if *telnetFlag != "" {
		deviceManager.SetDialer("relays", func() (io.ReadWriteCloser, error) { return telnet.DialTelnet(*telnetFlag) })
		slog.Info("dialing relays", "addr", *telnetFlag)
		dev, err := telnet.DialTelnet(*telnetFlag)
		if err != nil {
			return fmt.Errorf("failed to connect via telnet %s: %w", *telnetFlag, err)
		}
		deviceManager.SetDevice("relays", dev)
		slog.Info("connected to relays", "addr", *telnetFlag)
		modeStr = "telnet"
	} else {
		deviceManager.SetDialer("relays", func() (io.ReadWriteCloser, error) {
			c := &serial.Config{Name: *serialFlag, Baud: *baudFlag, ReadTimeout: time.Second}
			return serial.OpenPort(c)
		})
		slog.Info("dialing serial", "port", *serialFlag, "baud", *baudFlag)
		dev, err := serial.OpenPort(&serial.Config{Name: *serialFlag, Baud: *baudFlag, ReadTimeout: time.Second})
		if err != nil {
			return fmt.Errorf("failed to open serial %s@%d: %w", *serialFlag, *baudFlag, err)
		}
		deviceManager.SetDevice("relays", dev)
		slog.Info("opened serial", "port", *serialFlag, "baud", *baudFlag)
	}

	defer func() {
		if d := deviceManager.GetDevice("relays"); d != nil {
			_ = d.Close()
			slog.Info("closed device", "device", "relays")
		}
		if d := deviceManager.GetDevice("buzzer"); d != nil {
			_ = d.Close()
			slog.Info("closed device", "device", "buzzer")
		}
	}()

	// Start readers
	deviceManager.StartReader("relays")
	deviceManager.StartReader("buzzer")

	// Periodic status log
	go func() {
		t := time.NewTicker(StatusInterval)
		defer t.Stop()
		for range t.C {
			devs := map[string]bool{
				"relays": deviceManager.GetDevice("relays") != nil,
				"buzzer": deviceManager.GetDevice("buzzer") != nil,
			}
			states := deviceManager.RelayStates()

			// inline former formatRelayStatuses
			onCount := 0
			relayParts := make([]string, 0, len(states))
			for i, st := range states {
				ri := i + 1
				onOff := "OFF"
				if st.State {
					onOff = "ON"
					onCount++
				}
				if st.Label != "" {
					relayParts = append(relayParts, fmt.Sprintf("Relay %d (%s) %s", ri, st.Label, onOff))
				} else {
					relayParts = append(relayParts, fmt.Sprintf("Relay %d: %s", ri, onOff))
				}
			}
			relayStatuses := strings.Join(relayParts, " | ")

			deviceStatuses := make([]string, 0, len(devs))
			for k, v := range devs {
				if v {
					deviceStatuses = append(deviceStatuses, k+" 🟢")
				} else {
					deviceStatuses = append(deviceStatuses, k+" 🔴")
				}
			}

			slog.Info("status",
				"devices", strings.Join(deviceStatuses, " | "), // replaced stringsJoin
				"relays", relayStatuses,
				"relay_count", len(states),
				"on_count", onCount,
			)
		}
	}()

	adbClient := adb.NewClient() // use defaults; adjust in future if flags needed
	api := &router.API{Devices: deviceManager, ADB: adbClient}
	r := router.Router(api)

	addr := ":42069"
	slog.Info("server listening", "addr", addr, "mode", modeStr)
	if err := http.ListenAndServe(addr, r); err != nil {
		slog.Error("http server error", "err", err)
		return err
	}
	return nil
}

func main() {
	if err := Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
