package a2s

import "time"

const (
	defaultTimeout       = 3 * time.Second
	defaultPort          = 27015
	defaultMaxPacketSize = 4096
	minMaxPacketSize     = 1024
)

// Option mutates client construction settings.
type Option func(*clientConfig) error

type clientConfig struct {
	timeout       time.Duration
	maxPacketSize int
}

func defaultClientConfig() clientConfig {
	return clientConfig{
		timeout:       defaultTimeout,
		maxPacketSize: defaultMaxPacketSize,
	}
}

// WithTimeout sets the default query timeout.
func WithTimeout(d time.Duration) Option {
	return func(cfg *clientConfig) error {
		if d <= 0 {
			return newError(ErrorCodeTimeout, "with_timeout", "", "timeout must be greater than zero", nil)
		}
		cfg.timeout = d
		return nil
	}
}

// WithMaxPacketSize sets the maximum UDP packet buffer size.
func WithMaxPacketSize(size int) Option {
	return func(cfg *clientConfig) error {
		if size < minMaxPacketSize {
			return newError(ErrorCodePacketHeader, "with_max_packet_size", "", "max packet size must be at least 512 bytes", nil)
		}
		cfg.maxPacketSize = size
		return nil
	}
}
