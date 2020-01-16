package api

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/sirupsen/logrus"
	"github.com/ubccr/grendel/logger"
	"github.com/ubccr/grendel/model"
	"github.com/ubccr/grendel/util"
)

const (
	DefaultPort = 80
)

var log = logger.GetLogger("API")

type Server struct {
	ListenAddress net.IP
	ServerAddress net.IP
	SocketPath    string
	Port          int
	Scheme        string
	KeyFile       string
	CertFile      string
	Hostname      string
	DB            model.Datastore
}

func NewServer(db model.Datastore, socket, address string, port int) (*Server, error) {
	s := &Server{DB: db}

	if address == "" {
		address = net.IPv4zero.String()
	}

	if port == 0 {
		port = DefaultPort
	}

	s.Port = port

	ip := net.ParseIP(address)
	if ip == nil || ip.To4() == nil {
		return nil, fmt.Errorf("Invalid IPv4 address: %s", address)
	}

	s.ListenAddress = ip

	if !ip.To4().Equal(net.IPv4zero) {
		s.ServerAddress = ip
		return s, nil
	}

	ipaddr, err := util.GetInterfaceIP()
	if err != nil {
		return nil, err
	}

	s.ServerAddress = ipaddr
	s.SocketPath = socket

	return s, nil
}

func HTTPErrorHandler(err error, c echo.Context) {
	code := http.StatusInternalServerError
	if he, ok := err.(*echo.HTTPError); ok {
		code = he.Code
	}

	if code == http.StatusNotFound {
		log.WithFields(logrus.Fields{
			"path": c.Request().URL,
			"ip":   c.RealIP(),
		}).Error("Requested path not found")
	}

	c.String(code, "")
	c.Logger().Error(err)
}

func (s *Server) Serve() error {
	e := echo.New()
	e.HTTPErrorHandler = HTTPErrorHandler
	e.HideBanner = true
	e.Use(middleware.Recover())
	e.Logger = EchoLogger()
	e.Validator = &CustomValidator{validator: validator.New()}

	h, err := NewHandler(s.DB)
	if err != nil {
		return err
	}

	h.SetupRoutes(e)

	httpServer := &http.Server{
		ReadTimeout:  5 * time.Minute,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  120 * time.Second,
	}

	if s.SocketPath != "" {
		os.Remove(s.SocketPath)
		unixListener, err := net.Listen("unix", s.SocketPath)
		if err != nil {
			return err
		}
		e.Listener = unixListener
		s.Scheme = "http"
		log.Printf("Listening on unix domain socket: %s", s.SocketPath)
	} else if s.CertFile != "" && s.KeyFile != "" {
		cfg := &tls.Config{
			MinVersion: tls.VersionTLS12,
			CurvePreferences: []tls.CurveID{
				tls.CurveP256,
				tls.X25519,
			},
			PreferServerCipherSuites: true,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
		}

		httpServer.TLSConfig = cfg
		httpServer.TLSConfig.Certificates = make([]tls.Certificate, 1)
		httpServer.TLSConfig.Certificates[0], err = tls.LoadX509KeyPair(s.CertFile, s.KeyFile)
		httpServer.Addr = fmt.Sprintf("%s:%d", s.ListenAddress, s.Port)
		if err != nil {
			return err
		}

		s.Scheme = "https"
		log.Printf("Listening on %s://%s:%d", s.Scheme, s.ListenAddress, s.Port)
	} else {
		s.Scheme = "http"
		httpServer.Addr = fmt.Sprintf("%s:%d", s.ListenAddress, s.Port)
		log.Printf("Listening on %s://%s:%d", s.Scheme, s.ListenAddress, s.Port)
	}

	return e.StartServer(httpServer)
}
