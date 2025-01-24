package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-fuego/fuego"
	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
	"github.com/ubccr/grendel/pkg/model"
)

type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthSignupRequest struct {
	Username string `json:"username" validate:"required,min=2"`
	Password string `json:"password" validate:"required,min=8"`
}

type AuthResponse struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	Token    string `json:"token"`
	Expire   int64  `json:"expire"`
}

type AuthTokenRequest struct {
	Username string `json:"username" description:"username shown in logs, does not need to be a valid user in the DB" example:"user1:CLI"`
	Role     string `json:"role" description:"type of model.Role, valid options: disabled, user, admin" example:"admin"`
	Expire   string `json:"expire" description:"string parsed by time.ParseDuration, examples include: infinite, 8h, 30m, 20s" example:"infinite"`
}

type AuthTokenReponse struct {
	Token string `json:"token"`
}

var (
	expireDuration = time.Duration(8) * time.Hour
)

func (h *Handler) AuthSignin(c fuego.ContextWithBody[AuthRequest]) (*AuthResponse, error) {
	body, err := c.Body()
	if err != nil {
		return nil, fuego.HTTPError{
			Err:    err,
			Title:  "Authentication Error",
			Detail: "failed to parse auth body",
		}
	}

	authenticated, role, err := h.DB.VerifyUser(body.Username, body.Password)
	if err != nil {
		return nil, fuego.HTTPError{
			Err:    err,
			Title:  "Authentication Error",
			Detail: "failed to login, invalid credentials",
		}
	}
	if !authenticated {
		return nil, fuego.HTTPError{
			Err:    errors.New("invalid credentials"),
			Title:  "Authentication Error",
			Detail: "failed to login, invalid credentials",
		}
	}

	exp := time.Now().Add(expireDuration)

	token, err := CreateToken(body.Username, role, expireDuration)
	if err != nil {
		return nil, fuego.HTTPError{
			Err:    err,
			Title:  "Authentication Error",
			Detail: "failed to create token",
		}
	}

	c.SetCookie(http.Cookie{
		Name:     "Authorization",
		Value:    "Bearer " + token,
		Expires:  exp,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
	})

	return &AuthResponse{
		Username: body.Username,
		Role:     role,
		Expire:   exp.UnixMilli(),
	}, nil
}

func (h *Handler) AuthSignup(c fuego.ContextWithBody[AuthSignupRequest]) (*AuthResponse, error) {
	body, err := c.Body()
	if err != nil {
		return nil, fuego.HTTPError{
			Err:    err,
			Title:  "Authentication Error",
			Detail: "failed to parse auth body",
		}
	}

	role, err := h.DB.StoreUser(body.Username, body.Password)
	if err != nil {
		return nil, fuego.HTTPError{
			Err:    err,
			Title:  "Authentication Error",
			Detail: "failed to store user: " + body.Username,
		}
	}

	exp := time.Now().Add(expireDuration)
	token, err := CreateToken(body.Username, role, expireDuration)
	if err != nil {
		return nil, fuego.HTTPError{
			Err:    err,
			Title:  "Authentication Error",
			Detail: "failed to create token",
		}
	}

	c.SetCookie(http.Cookie{
		Name:     "Authorization",
		Value:    "Bearer " + token,
		Expires:  exp,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
	})

	return &AuthResponse{
		Username: body.Username,
		Role:     role,
		Expire:   exp.UnixMilli(),
	}, nil
}

func (h *Handler) AuthSignout(c fuego.ContextNoBody) (*GenericResponse, error) {
	c.SetCookie(http.Cookie{
		Name:     "Authorization",
		Value:    "",
		Expires:  time.Now(),
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
	})

	return &GenericResponse{
		Title:   "Success",
		Detail:  "successfully signed out",
		Changed: 1,
	}, nil
}

func (h *Handler) AuthToken(c fuego.ContextWithBody[AuthTokenRequest]) (*AuthTokenReponse, error) {
	body, err := c.Body()
	if err != nil {
		return nil, fuego.HTTPError{
			Err:    err,
			Title:  "Authentication Error",
			Detail: "failed to parse auth body",
		}
	}

	var exp time.Duration
	if body.Expire == "infinite" {
		exp = -1
	} else {
		exp, err = time.ParseDuration(body.Expire)
		if err != nil {
			return nil, fuego.HTTPError{
				Err:    err,
				Title:  "Authentication Error",
				Detail: "failed to parse expire time",
			}
		}
	}

	token, err := CreateToken(body.Username, body.Role, exp)
	if err != nil {
		return nil, fuego.HTTPError{
			Err:    err,
			Title:  "Authentication Error",
			Detail: "failed to create token",
		}
	}

	return &AuthTokenReponse{
		Token: token,
	}, nil
}

func CreateToken(username, role string, expire time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"alg":      jwt.SigningMethodHS256,
		"username": username,
		"role":     role,
		"iss":      "grendel",
		"nbf":      time.Date(2015, 10, 10, 12, 0, 0, 0, time.UTC).Unix(),
	}

	if expire != -1 {
		claims["exp"] = time.Now().Add(expire).Unix()
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(viper.GetString("api.secret")))

}

func VerifyToken(tokenString string) (bool, string, string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(viper.GetString("api.secret")), nil
	})
	if err != nil {
		return false, "", "", err
	}

	username := ""
	role := model.RoleDisabled.String()

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		username = claims["username"].(string)
		role = claims["role"].(string)

		expTime, err := claims.GetExpirationTime()
		if err != nil {
			return false, "", "", errors.New("failed to parse expire time")
		}

		if expTime != nil {
			comp := time.Now().Compare(expTime.Time)
			if comp == 1 {
				return false, "", "", errors.New("token is expired")
			}
		}

	} else {
		return false, "", "", errors.New("failed to extract claims")
	}

	return true, username, role, nil
}
