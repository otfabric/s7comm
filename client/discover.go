package client

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// DiscoverResult holds a single discovery result
type DiscoverResult struct {
	IP      string
	Port    int
	IsS7    bool
	Rack    int
	Slot    int
	PDUSize int
	TSAP    string
	Error   string
}

// DiscoverOption configures discovery
type DiscoverOption func(*discoverOpts)

type discoverOpts struct {
	timeout  int
	parallel int
	rackMin  int
	rackMax  int
	slotMin  int
	slotMax  int
	rateMS   int
}

// WithDiscoverTimeout sets per-host timeout in ms
func WithDiscoverTimeout(ms int) DiscoverOption {
	return func(o *discoverOpts) { o.timeout = ms }
}

// WithDiscoverParallel sets parallel probe count
func WithDiscoverParallel(n int) DiscoverOption {
	return func(o *discoverOpts) { o.parallel = n }
}

// WithDiscoverRackSlotRange sets rack and slot probe ranges (inclusive).
func WithDiscoverRackSlotRange(rackMin, rackMax, slotMin, slotMax int) DiscoverOption {
	return func(o *discoverOpts) {
		o.rackMin = rackMin
		o.rackMax = rackMax
		o.slotMin = slotMin
		o.slotMax = slotMax
	}
}

// WithDiscoverRateLimit sets delay between rack/slot probes in milliseconds.
func WithDiscoverRateLimit(ms int) DiscoverOption {
	return func(o *discoverOpts) { o.rateMS = ms }
}

// Discover scans a CIDR range for S7 devices
func Discover(ctx context.Context, cidr string, opts ...DiscoverOption) ([]DiscoverResult, error) {
	dOpts := &discoverOpts{
		timeout:  2000,
		parallel: 10,
		rackMin:  0,
		rackMax:  3,
		slotMin:  0,
		slotMax:  5,
		rateMS:   0,
	}
	for _, opt := range opts {
		opt(dOpts)
	}

	ips, err := expandCIDR(cidr)
	if err != nil {
		return nil, err
	}

	results := make([]DiscoverResult, 0)
	var mu sync.Mutex
	var wg sync.WaitGroup
	if dOpts.parallel < 1 {
		dOpts.parallel = 1
	}

	jobs := make(chan string)
	for i := 0; i < dOpts.parallel; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case ip, ok := <-jobs:
					if !ok {
						return
					}
					result := probeS7(ctx, ip, 102, dOpts)
					mu.Lock()
					results = append(results, result)
					mu.Unlock()
				}
			}
		}()
	}

	for _, ip := range ips {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return results, ctx.Err()
		case jobs <- ip:
		}
	}
	close(jobs)

	wg.Wait()
	if err := ctx.Err(); err != nil {
		return results, err
	}
	return results, nil
}

func expandCIDR(cidr string) ([]string, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	var ips []string
	for ip := ipnet.IP.Mask(ipnet.Mask); ipnet.Contains(ip); incIP(ip) {
		ips = append(ips, ip.String())
	}

	// Remove network and broadcast for /24 and larger
	if len(ips) > 2 {
		ips = ips[1 : len(ips)-1]
	}
	return ips, nil
}

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func probeS7(ctx context.Context, ip string, port int, opts *discoverOpts) DiscoverResult {
	result := DiscoverResult{IP: ip, Port: port}
	if !isPortOpen(ctx, ip, port) {
		result.Error = "port 102 closed"
		return result
	}

	for rack := opts.rackMin; rack <= opts.rackMax; rack++ {
		for slot := opts.slotMin; slot <= opts.slotMax; slot++ {
			if err := ctx.Err(); err != nil {
				result.Error = err.Error()
				return result
			}
			if opts.rateMS > 0 {
				select {
				case <-ctx.Done():
					result.Error = ctx.Err().Error()
					return result
				case <-time.After(time.Duration(opts.rateMS) * time.Millisecond):
				}
			}

			c := New(ip,
				WithPort(port),
				WithRackSlot(rack, slot),
				WithTimeout(time.Duration(opts.timeout)*time.Millisecond),
			)

			if err := c.Connect(ctx); err != nil {
				continue
			}

			info := c.ConnectionInfo()
			_ = c.Close()

			result.IsS7 = true
			result.Rack = info.Rack
			result.Slot = info.Slot
			result.PDUSize = info.PDUSize
			result.TSAP = fmt.Sprintf("0x%04X", info.RemoteTSAP)
			return result
		}
	}

	result.Error = "no valid COTP+S7 setup for tested rack/slot range"
	return result
}

func isPortOpen(ctx context.Context, ip string, port int) bool {
	dialCtx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(dialCtx, "tcp", net.JoinHostPort(ip, fmt.Sprint(port)))
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
