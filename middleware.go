package loggermiddleware

import (
	"bytes"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/gen1us2k/log"
	"github.com/labstack/echo"
	"github.com/valyala/fasttemplate"
)

type (
	// LoggerConfig defines the config for logger middleware.
	LoggerConfig struct {
		// Log format which can be constructed using the following tags:
		//
		// - time_rfc3339
		// - id (Request ID - Not implemented)
		// - remote_ip
		// - uri
		// - host
		// - method
		// - path
		// - referer
		// - user_agent
		// - status
		// - latency (In microseconds)
		// - latency_human (Human readable)
		// - rx_bytes (Bytes received)
		// - tx_bytes (Bytes sent)
		//
		// Example "${remote_ip} ${status}"
		//
		// Optional. Default value DefaultLoggerConfig.Format.
		Format string `json:"format"`

		// Logger is a writer where logs are written.
		Logger log.Logger

		template   *fasttemplate.Template
		bufferPool sync.Pool
	}
)

var (
	// DefaultLoggerConfig is the default logger middleware config.
	DefaultLoggerConfig = LoggerConfig{
		Format: `${remote_ip} - - "${method} ${url}" ${status} ${tx_bytes} ` +
			`${referer} ${user_agent}` + "\n",
	}
)

// CustomLogger returns a middleware that logs HTTP requests.
func CustomLogger() echo.MiddlewareFunc {
	return LoggerWithConfig(DefaultLoggerConfig)
}

// LoggerWithConfig returns a logger middleware from config.
// See: `Logger()`.
func LoggerWithConfig(config LoggerConfig) echo.MiddlewareFunc {
	// Defaults
	if config.Format == "" {
		config.Format = DefaultLoggerConfig.Format
	}
	if config.Logger == nil {
		config.Logger = log.NewLogger("http_server")
	}

	config.template = fasttemplate.New(config.Format, "${", "}")
	config.bufferPool = sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 256))
		},
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			req := c.Request()
			res := c.Response()
			start := time.Now()
			if err = next(c); err != nil {
				c.Error(err)
			}
			stop := time.Now()
			buf := config.bufferPool.Get().(*bytes.Buffer)
			buf.Reset()
			defer config.bufferPool.Put(buf)

			_, err = config.template.ExecuteFunc(buf, func(w io.Writer, tag string) (int, error) {
				switch tag {
				case "time_rfc3339":
					return w.Write([]byte(time.Now().Format(time.RFC3339)))
				case "remote_ip":
					ra := req.RemoteAddress()
					if ip := req.Header().Get(echo.HeaderXRealIP); ip != "" {
						ra = ip
					} else if ip = req.Header().Get(echo.HeaderXForwardedFor); ip != "" {
						ra = ip
					} else {
						ra, _, _ = net.SplitHostPort(ra)
					}
					return w.Write([]byte(ra))
				case "host":
					return w.Write([]byte(req.Host()))
				case "uri":
					return w.Write([]byte(req.URI()))
				case "method":
					return w.Write([]byte(req.Method()))
				case "path":
					p := req.URL().Path()
					if p == "" {
						p = "/"
					}
					return w.Write([]byte(p))
				case "referer":
					return w.Write([]byte(req.Referer()))
				case "user_agent":
					return w.Write([]byte(req.UserAgent()))
				case "status":
					return w.Write([]byte(strconv.Itoa(res.Status())))
				case "latency":
					l := stop.Sub(start).Nanoseconds() / 1000
					return w.Write([]byte(strconv.FormatInt(l, 10)))
				case "latency_human":
					return w.Write([]byte(stop.Sub(start).String()))
				case "rx_bytes":
					b := req.Header().Get(echo.HeaderContentLength)
					if b == "" {
						b = "0"
					}
					return w.Write([]byte(b))
				case "tx_bytes":
					return w.Write([]byte(strconv.FormatInt(res.Size(), 10)))
				}
				return 0, nil
			})
			if err == nil {
				config.Logger.Info(string(buf.Bytes()))
			}
			return
		}
	}
}
