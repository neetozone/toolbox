package storage

import (
	"context"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/retry"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	log "github.com/sirupsen/logrus"
)

// HTTPTimeoutEnvVar overrides the response-header and stall timeouts, in seconds.
const HTTPTimeoutEnvVar = "NEETO_CI_CACHE_HTTP_TIMEOUT"

const (
	dialTimeout           = 10 * time.Second
	tlsHandshakeTimeout   = 10 * time.Second
	defaultHeaderTimeout  = 30 * time.Second
	defaultStallTimeout   = 60 * time.Second
	s3MaxAttempts         = 3
	s3MaxBackoff          = 5 * time.Second
	maxStallConnWriteSize = 64 * 1024
)

type httpTimeouts struct {
	Header time.Duration
	Stall  time.Duration
}

func httpTimeoutsFromEnv() httpTimeouts {
	value := os.Getenv(HTTPTimeoutEnvVar)
	if value == "" {
		return httpTimeouts{Header: defaultHeaderTimeout, Stall: defaultStallTimeout}
	}

	seconds, err := strconv.Atoi(value)
	if err != nil || seconds <= 0 {
		log.Errorf("Ignoring %s='%s': not a positive number of seconds.", HTTPTimeoutEnvVar, value)
		return httpTimeouts{Header: defaultHeaderTimeout, Stall: defaultStallTimeout}
	}

	timeout := time.Duration(seconds) * time.Second
	return httpTimeouts{Header: timeout, Stall: timeout}
}

// withS3Timeouts bounds every S3 request; a transfer only fails when it stops making progress.
func withS3Timeouts(o *s3.Options) {
	o.HTTPClient = newS3HTTPClient(httpTimeoutsFromEnv())
	o.Retryer = retry.NewStandard(func(so *retry.StandardOptions) {
		so.MaxAttempts = s3MaxAttempts
		so.MaxBackoff = s3MaxBackoff
	})
}

func newS3HTTPClient(timeouts httpTimeouts) *awshttp.BuildableClient {
	return awshttp.NewBuildableClient().
		WithDialerOptions(func(d *net.Dialer) {
			d.Timeout = dialTimeout
		}).
		WithTransportOptions(func(tr *http.Transport) {
			tr.TLSHandshakeTimeout = tlsHandshakeTimeout
			tr.ResponseHeaderTimeout = timeouts.Header
			dial := tr.DialContext
			tr.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
				conn, err := dial(ctx, network, address)
				if err != nil {
					return nil, err
				}

				return &stallConn{Conn: conn, timeout: timeouts.Stall}, nil
			}
		})
}

// stallConn fails I/O only after timeout without progress; slow transfers keep going.
type stallConn struct {
	net.Conn
	timeout time.Duration
}

func (c *stallConn) Read(b []byte) (int, error) {
	if err := c.Conn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		return 0, err
	}

	return c.Conn.Read(b)
}

func (c *stallConn) Write(b []byte) (int, error) {
	written := 0
	for written < len(b) {
		end := written + maxStallConnWriteSize
		if end > len(b) {
			end = len(b)
		}

		if err := c.Conn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
			return written, err
		}

		n, err := c.Conn.Write(b[written:end])
		written += n
		if err != nil {
			return written, err
		}
	}

	return written, nil
}
