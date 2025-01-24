package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-fuego/fuego"
	"github.com/rs/cors"
	"github.com/spf13/viper"
	"github.com/ubccr/grendel/pkg/model"
)

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// skip auth if bound to unix socket
		if viper.IsSet("api.socket_path") {
			next.ServeHTTP(w, r)
			return
		}

		var token string
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			token = authHeader
		}
		authCookie, _ := r.Cookie("Authorization")
		if authCookie != nil {
			token = authCookie.Value
		}

		if token == "" {
			err := fmt.Errorf("authentication error ip=%s", r.RemoteAddr)
			fuego.SendError(w, r, fuego.UnauthorizedError{
				Err:    err,
				Title:  "Error",
				Detail: "failed to authenticate",
			})
			return
		}

		authenticated, username, role, err := VerifyToken(strings.TrimPrefix(token, "Bearer "))
		if err != nil {
			err := fmt.Errorf("authentication error ip=%s err=%s", r.RemoteAddr, err)
			fuego.SendError(w, r, fuego.HTTPError{
				Err:    err,
				Title:  "Error",
				Detail: "failed to verify token",
			})
			return
		}
		if !authenticated {
			err := fmt.Errorf("authentication error ip=%s", r.RemoteAddr)
			fuego.SendError(w, r, fuego.HTTPError{
				Err:    err,
				Title:  "Error",
				Detail: "failed to verify token",
			})
			return
		}
		if role == model.RoleDisabled.String() {
			err := fmt.Errorf("authentication error, account is disabled ip=%s, user=%s", r.RemoteAddr, username)
			fuego.SendError(w, r, fuego.HTTPError{
				Err:    err,
				Title:  "Error",
				Detail: "account is disabled",
			})
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, "username", username)
		ctx = context.WithValue(ctx, "role", role)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Debugf("api request: method=%s route=%s ip=%s", r.Method, r.URL, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

func corsMiddleware(enabled bool) func(h http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if enabled {
				cors.New(cors.Options{
					AllowedOrigins:   []string{"*"},
					AllowedMethods:   []string{"GET", "PATCH", "POST", "PUT", "DELETE", "OPTIONS"},
					AllowedHeaders:   []string{"*"},
					AllowCredentials: true,
				}).HandlerFunc(w, r)
			}
			next.ServeHTTP(w, r)
		})
	}
}
