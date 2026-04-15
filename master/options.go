package master

import "time"

const (
	defaultTimeout       = 3 * time.Second
	defaultPort          = 27011
	defaultBaseAddress   = "hl2master.steampowered.com:27011"
	defaultMaxPacketSize = 1400
	minMaxPacketSize     = 64
)

// Option mutates client construction settings.
type Option func(*clientConfig) error

type clientConfig struct {
	timeout       time.Duration
	baseAddress   string
	maxPacketSize int
}

func defaultClientConfig() clientConfig {
	return clientConfig{
		timeout:       defaultTimeout,
		baseAddress:   defaultBaseAddress,
		maxPacketSize: defaultMaxPacketSize,
	}
}

// WithTimeout sets the default discovery timeout.
func WithTimeout(d time.Duration) Option {
	return func(cfg *clientConfig) error {
		if d <= 0 {
			return newError(ErrorCodeTimeout, "with_timeout", "", "timeout must be greater than zero", nil)
		}
		cfg.timeout = d
		return nil
	}
}

// WithBaseAddress overrides the default master server address.
func WithBaseAddress(addr string) Option {
	return func(cfg *clientConfig) error {
		cfg.baseAddress = addr
		return nil
	}
}

// WithMaxPacketSize sets the maximum UDP packet buffer size.
func WithMaxPacketSize(size int) Option {
	return func(cfg *clientConfig) error {
		if size < minMaxPacketSize {
			return newError(ErrorCodePacketHeader, "with_max_packet_size", "", "max packet size must be at least 64 bytes", nil)
		}
		cfg.maxPacketSize = size
		return nil
	}
}
