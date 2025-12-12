package telnet

import (
	"context"
	"io"
	"log/slog"
	"net"
	"time"
)

const DefaultTelnetPort = "23"

func DialTelnet(addr string) (io.ReadWriteCloser, error) {
	host, port, perr := net.SplitHostPort(addr)
	if perr != nil {
		host = addr
		port = DefaultTelnetPort
	}
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
		Resolver: &net.Resolver{
			PreferGo: false,
		},
	}
	target := net.JoinHostPort(host, port)

	slog.Info("telnet dialing", "target", target)
	start := time.Now()
	conn, err := dialer.DialContext(context.Background(), "tcp", target)
	dur := time.Since(start)
	if err == nil {
		slog.Info("telnet connected", "target", target, "dur", dur)
		return conn, nil
	}

	slog.Warn("telnet primary dial failed; attempting DNS fallbacks", "target", target, "err", err)
	ips, lerr := net.LookupHost(host)
	if lerr == nil && len(ips) > 0 {
		slog.Info("telnet resolved host", "host", host, "ips", ips)
		var lastErr error
		for i, ip := range ips {
			tryAddr := net.JoinHostPort(ip, port)
			slog.Info("telnet dialing resolved IP", "addr", tryAddr, "index", i)
			ipStart := time.Now()
			conn, lastErr = dialer.DialContext(context.Background(), "tcp", tryAddr)
			ipDur := time.Since(ipStart)
			if lastErr == nil {
				slog.Info("telnet connected (resolved IP)", "addr", tryAddr, "dur", ipDur)
				return conn, nil
			}
			slog.Warn("telnet dial to resolved IP failed", "addr", tryAddr, "err", lastErr, "dur", ipDur)
		}
		if lastErr != nil {
			err = lastErr
		}
	}
	return nil, err
}
