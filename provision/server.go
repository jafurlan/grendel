// Copyright 2019 Grendel Authors. All rights reserved.
//
// This file is part of Grendel.
//
// Grendel is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Grendel is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with Grendel. If not, see <https://www.gnu.org/licenses/>.

package provision

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

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

var log = logger.GetLogger("PROVISION")

type Server struct {
	ListenAddress net.IP
	ServerAddress net.IP
	Port          int
	Scheme        string
	KeyFile       string
	CertFile      string
	RepoDir       string
	DB            model.DataStore
	httpServer    *http.Server
}

func NewServer(db model.DataStore, address string) (*Server, error) {
	s := &Server{DB: db}

	shost, sport, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	if shost == "" {
		shost = net.IPv4zero.String()
	}

	port := DefaultPort
	if sport != "" {
		var err error
		port, err = strconv.Atoi(sport)
		if err != nil {
			return nil, err
		}
	}

	s.Port = port

	ip := net.ParseIP(shost)
	if ip == nil || ip.To4() == nil {
		return nil, fmt.Errorf("Invalid IPv4 address: %s", shost)
	}

	s.ListenAddress = ip

	if !ip.To4().Equal(net.IPv4zero) {
		s.ServerAddress = ip
		return s, nil
	}

	ipaddr, err := util.GetFirstExternalIPFromInterfaces()
	if err != nil {
		return nil, err
	}

	s.ServerAddress = ipaddr

	return s, nil
}

func newEcho() (*echo.Echo, error) {
	e := echo.New()
	e.HTTPErrorHandler = HTTPErrorHandler
	e.HideBanner = true
	e.Use(middleware.Recover())
	e.Logger = EchoLogger()

	renderer, err := NewTemplateRenderer()
	if err != nil {
		return nil, err
	}

	e.Renderer = renderer

	return e, nil
}

func HTTPErrorHandler(err error, c echo.Context) {
	path := c.Request().URL.Path
	if he, ok := err.(*echo.HTTPError); ok {
		if he.Code == http.StatusNotFound {
			log.WithFields(logrus.Fields{
				"path": path,
				"ip":   c.RealIP(),
			}).Warn("Requested path not found")
		} else {
			log.WithFields(logrus.Fields{
				"code": he.Code,
				"err":  he.Internal,
				"path": path,
				"ip":   c.RealIP(),
			}).Error(he.Message)
		}
	} else {
		log.WithFields(logrus.Fields{
			"err":  err,
			"path": path,
			"ip":   c.RealIP(),
		}).Error("HTTP Error")
	}

	c.Echo().DefaultHTTPErrorHandler(err, c)
}

func (s *Server) Serve(defaultImageName string) error {
	e, err := newEcho()
	if err != nil {
		return err
	}

	routeList, err := json.MarshalIndent(e.Routes(), "", "  ")
	if err != nil {
		return err
	}
	log.Infof("Routes in order: %s", routeList)

	log.Infof("Using repo dir: %s", s.RepoDir)

	if len(s.RepoDir) > 0 {
		log.Infof("Using repo dir: %s", s.RepoDir)
		e.Static("/repo", s.RepoDir)
		//fs := http.FileServer(http.Dir(s.RepoDir))
		//e.GET("/repo/*", echo.WrapHandler(http.StripPrefix("/repo/", fs)))
	}

	h, err := NewHandler(s.DB, defaultImageName)
	if err != nil {
		return err
	}

	h.SetupRoutes(e)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.ListenAddress, s.Port),
		ReadTimeout:  60 * time.Minute,
		WriteTimeout: 60 * time.Minute,
		IdleTimeout:  120 * time.Second,
	}

	if s.CertFile != "" && s.KeyFile != "" {
		cfg := &tls.Config{
			MinVersion: tls.VersionTLS12,
			/* TODO need to figure out compataible ciphers with iPXE

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
				tls.TLS_RSA_WITH_AES_256_CBC_SHA256,
			},
			*/
		}

		httpServer.TLSConfig = cfg
		httpServer.TLSConfig.Certificates = make([]tls.Certificate, 1)
		httpServer.TLSConfig.Certificates[0], err = tls.LoadX509KeyPair(s.CertFile, s.KeyFile)
		if err != nil {
			return err
		}

		s.Scheme = "https"
		httpServer.Addr = fmt.Sprintf("%s:%d", s.ListenAddress, s.Port)
	} else {
		s.Scheme = "http"
	}

	s.httpServer = httpServer
	log.Infof("Listening on %s://%s:%d", s.Scheme, s.ListenAddress, s.Port)
	if err := e.StartServer(httpServer); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}

	return s.httpServer.Shutdown(ctx)
}
