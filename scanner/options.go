package scanner

import "time"

const (
	defaultConcurrency   = 32
	defaultTimeout       = 3 * time.Second
	defaultMaxPacketSize = 4096
	minMaxPacketSize     = 1024
)

// Option mutates scanner construction settings.
type Option func(*clientConfig) error

type clientConfig struct {
	concurrency   int
	timeout       time.Duration
	maxPacketSize int
}

func defaultClientConfig() clientConfig {
	return clientConfig{
		concurrency:   defaultConcurrency,
		timeout:       defaultTimeout,
		maxPacketSize: defaultMaxPacketSize,
	}
}

// WithConcurrency sets the worker count used by Probe.
func WithConcurrency(n int) Option {
	return func(cfg *clientConfig) error {
		if n <= 0 {
			return newError(ErrorCodeConcurrency, "with_concurrency", zeroServer, "concurrency must be greater than zero", nil)
		}
		cfg.concurrency = n
		return nil
	}
}

// WithTimeout sets the default probe timeout.
func WithTimeout(d time.Duration) Option {
	return func(cfg *clientConfig) error {
		if d <= 0 {
			return newError(ErrorCodeTimeout, "with_timeout", zeroServer, "timeout must be greater than zero", nil)
		}
		cfg.timeout = d
		return nil
	}
}

// WithMaxPacketSize sets the maximum UDP packet buffer size for one probe.
func WithMaxPacketSize(size int) Option {
	return func(cfg *clientConfig) error {
		if size < minMaxPacketSize {
			return newError(ErrorCodePacketSize, "with_max_packet_size", zeroServer, "max packet size must be at least 1024 bytes", nil)
		}
		cfg.maxPacketSize = size
		return nil
	}
}
