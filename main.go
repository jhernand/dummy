package main

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Default data and buffer sizes.
const (
	defaultDataSize   = 1 * (1 << 30)  // 1 GiB
	defaultBufferSize = 32 * (1 << 10) // 32 KiB
)

// Default listen address:
const (
	defaultListenAddress = ":8443"
)

// Handler is an HTTP handler that sends random data. The 'size' query parameter determines the total amount of bytes to
// send. The 'buffer' quer parameter determines the size of the buffer used internally.
type Handler struct {
	logger *slog.Logger
}

// ServeHTTP is the implementation of the http.Handler interface.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var err error

	// Get the current time so that we can later measure the elapsed time:
	startTime := time.Now()

	// Write to the log the details of the request:
	h.logger.Info(
		"Received request",
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.Any("query", r.URL.Query()),
		slog.Any("headers", r.Header),
	)

	// Get the response size:
	dataSize := defaultDataSize
	text := r.URL.Query().Get("size")
	if text != "" {
		value, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			h.logger.Error(
				"Failed to parse response size query paramer",
				slog.String("value", text),
				slog.String("error", err.Error()),
			)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		dataSize = int(value)
	}
	h.logger.Info(
		"Response size",
		slog.Int("size", dataSize),
	)

	// Get the buffer size:
	bufferSize := defaultBufferSize
	text = r.URL.Query().Get("buffer")
	if text != "" {
		value, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			h.logger.Error(
				"Failed to parse buffer size query paramer",
				slog.String("value", text),
				slog.String("error", err.Error()),
			)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		bufferSize = int(value)
	}
	h.logger.Info(
		"Buffer size",
		slog.Int("size", bufferSize),
	)

	// Open the file:
	dataFile, err := os.Open("/dev/urandom")
	if err != nil {
		h.logger.Error(
			"Failed to open data file",
			slog.String("error", err.Error()),
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer func() {
		err := dataFile.Close()
		if err != nil {
			h.logger.Error(
				"Failed to close data file",
				slog.String("error", err.Error()),
			)
		}
	}()

	// Send the data:
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	dataBuffer := make([]byte, bufferSize)
	pendingSize := dataSize
	for pendingSize > 0 {
		var readSize int
		if pendingSize > bufferSize {
			readSize = bufferSize
		} else {
			readSize = pendingSize
		}
		readBuffer := dataBuffer[0:readSize]
		n, err := dataFile.Read(readBuffer)
		if err != nil {
			h.logger.Error(
				"Failed to read data",
				slog.Int("size", readSize),
				slog.String("error", err.Error()),
			)
			return
		}
		if n != len(readBuffer) {
			h.logger.Error(
				"Unexpected read size",
				slog.Int("expected", readSize),
				slog.Int("actual", n),
			)
			return
		}
		n, err = w.Write(readBuffer)
		if err != nil {
			h.logger.Error(
				"Failed to write data",
				slog.Int("size", readSize),
				slog.String("error", err.Error()),
			)
			return
		}
		if n != len(readBuffer) {
			h.logger.Error(
				"Unexpected write size",
				slog.Int("expected", readSize),
				slog.Int("actual", n),
			)
			return
		}
		pendingSize -= readSize
	}

	// Calculate the elapsedTime time:
	elapsedTime := time.Since(startTime)

	// Write a summary to the log:
	h.logger.Info(
		"Data sent",
		slog.Int("size", dataSize),
		slog.Int("buffer", bufferSize),
		slog.String("elapsed", elapsedTime.String()),
	)
}

func main() {
	// Prepare the logger:
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Create the handler:
	handler := &Handler{
		logger: logger,
	}

	// Create temporary files for the TLS certificate and key:
	tlsDir, err := os.MkdirTemp("", ".tls")
	if err != nil {
		logger.Error(
			"Failed to create temporary TLS directory",
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}
	tlsCrtFile := filepath.Join(tlsDir, "tls.crt")
	err = os.WriteFile(tlsCrtFile, []byte(tlsCrt), 0o600)
	if err != nil {
		logger.Error(
			"Failed to create TLS certificate file",
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}
	tlsKeyFile := filepath.Join(tlsDir, "tls.key")
	err = os.WriteFile(tlsKeyFile, []byte(tlsKey), 0o600)
	if err != nil {
		logger.Error(
			"Failed to create TLS key file",
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}

	// Start the server:
	logger.Info(
		"Ready to listen and serve",
		"address", defaultListenAddress,
	)
	err = http.ListenAndServeTLS(defaultListenAddress, tlsCrtFile, tlsKeyFile, handler)
	if err != nil {
		slog.Error(
			"Failed to listen and serve",
			slog.String("error", err.Error()),
		)
	}
}

// These certificate and private key file are intended only for rests, and can be regeneraed with a command like this:
//
//	openssl req \
//	-x509 \
//	-newkey rsa:4096 \
//	-nodes \
//	-keyout tls.key \
//	-out tls.crt \
//	-subj '/CN=my-service.my-namespace.svc.cluster.local' \
//	-addext 'subjectAltName=DNS:my-service.my-namespace.svc.cluster.local' \
//	-days 365
const tlsCrt = `
-----BEGIN CERTIFICATE-----
MIIFgTCCA2mgAwIBAgIUdxRaoy5YrUdH6DOXCqrt+417vKQwDQYJKoZIhvcNAQEL
BQAwNDEyMDAGA1UEAwwpbXktc2VydmljZS5teS1uYW1lc3BhY2Uuc3ZjLmNsdXN0
ZXIubG9jYWwwHhcNMjQwOTI1MTQwMzM1WhcNMjUwOTI1MTQwMzM1WjA0MTIwMAYD
VQQDDClteS1zZXJ2aWNlLm15LW5hbWVzcGFjZS5zdmMuY2x1c3Rlci5sb2NhbDCC
AiIwDQYJKoZIhvcNAQEBBQADggIPADCCAgoCggIBALFV2/bioz576TFxqbNg1Ml5
L620XanxU5XHZakiWPoAV8shKByn/vP5Pmhsd18kqtrxyLBbFDdX/H99xH5uxhnj
So5POOAJi5KGXUxa53cZ3FNkld+Qx1cINpWDAw0CD0u7pS9sByr85C23hoWU9nH0
uf6ZhUV+oB+hScLtoFUs9M3R3CErvzl4ObWFvngIlMsuGK3H6/Z2GizMskSZ9Kke
OAtV0Q+dsQn4dDuWJ7GMzd9UqJ82D3kE1d5yxlKN8dKPmAVJZ5EZNd4NyRtiotYJ
VfPszsoixjddAOiHc3E3WPfJPXU1PBzzCKZy9+qDytRhtt5GwwjGQPBOH68LaFgQ
OiCAIEleUziPFRY0fbFGBN9N9HS/jPiS72QxBEF0O8xWozvJB47EqLHxJzvJE7u4
AobD3pGKnurg139WAuVhzvCcnTYMFGtrwd0a21TiNnM0AB/ob91s84sjI9MXOfrt
jP4HwL3xSfCxrFS2qGSL54FYEjIvgwlH4h1MkXvxrezVQAJV1v1EhQRrqtLFM454
pBtXKBgnc4OlgsoNPNBdMHroKK0tTa5H77a953jODVv6oxG9cQBCGESVHff0OP/V
S2+jeUQfpaydewYtiVqRwDhPQhqnI0rxYCgw6s9SRFaBXdeXXSoDjx8BpQgbL0px
55XtwecxX5mctOIgzFfbAgMBAAGjgYowgYcwHQYDVR0OBBYEFFRDc6lRbXO7u4n4
6tzTT2QRbQ/QMB8GA1UdIwQYMBaAFFRDc6lRbXO7u4n46tzTT2QRbQ/QMA8GA1Ud
EwEB/wQFMAMBAf8wNAYDVR0RBC0wK4IpbXktc2VydmljZS5teS1uYW1lc3BhY2Uu
c3ZjLmNsdXN0ZXIubG9jYWwwDQYJKoZIhvcNAQELBQADggIBAHPxmbkDXmj07WVu
VWlUdMFYJdHxWpgLYcsXsgcwavOOy9bZ7IgoWwgkASfXc13E/72IkpCxAUuyjpqU
bj3xvKRKxvEMfHq0NyxiYbLTFQRorfH2POwOXS1uc3clwYgXVUoM3Y3U2XAfAGEv
n4H5c7we6DbLrkplmFxCmsX2qElppXzGVherrml/+wy7guGKD75QB42GBGkOoot5
z3cW83w/xesofPIZ0Rpyg3MNGxp+0VPsSAuSIn/6bd1H3BmoRiHu5Ms4y7P7aV91
64jVRt3/pvp1ZzJbCTH9plaHkC5FkwP1xj5mxQEIVIMMqlBjzqynQngeWIzT5hDl
Ub4nPyX3V/CJsRuWSTF1oqb43q/BLOrcUQ3LxYZbIeDwrzsfz9NngBfC9e1JHoJ1
l+QvF+dJKi4ejMTzjsO79YwlEzXpbQUFBY+KjLNEhAlUVpvT8/T29aa55AtoPAS9
lr6qpcA2HBQLWzHzLECG9OI0uGE0iRSR7vxsrQkxFrZndOuKUlsMRoIEZihv1e4z
adIzKv1etyXZRNzo1eLzGtkbNSmpj0y6W06qAwcYm8b3o1Y4/zf0+gGaelmwf+ic
FYOQh5rSVQTzveIfWu6CjYcHbi+E5K01nOmxjIrZB0vG5arRRJqSM4bJUw4NobAL
fWTzUCEP6tO958N2HQPKrhOuUDFx
-----END CERTIFICATE-----
`

const tlsKey = `
-----BEGIN PRIVATE KEY-----
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
L3A+HtEIwSkaS2iyKP5X9IBZVByg
-----END PRIVATE KEY-----
`
