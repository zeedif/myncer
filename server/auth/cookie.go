package auth

import (
	"net/http"
	"time"

	myncer_pb "github.com/hansbala/myncer/proto/myncer"
)

func isSecure(mode myncer_pb.ServerMode) bool {
	switch mode {
	case myncer_pb.ServerMode_DEV:
		return false
	default:
		return true
	}
}

func GetAuthCookie(jwtToken string, serverMode myncer_pb.ServerMode) *http.Cookie {
	return &http.Cookie{
		Name:     cJwtCookieName,
		Value:    jwtToken,
		Path:     "/",
		HttpOnly: isHttpOnly(serverMode),
		Secure:   isSecure(serverMode),
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now().Add(24 * time.Hour),
	}
}

func GetLogoutAuthCookie(serverMode myncer_pb.ServerMode) *http.Cookie {
	return &http.Cookie{
		Name:     cJwtCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: isHttpOnly(serverMode),
		Secure:   isSecure(serverMode),
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	}
}

func SetAuthCookie(w http.ResponseWriter, jwtToken string, serverMode myncer_pb.ServerMode) {
	http.SetCookie(w, GetAuthCookie(jwtToken, serverMode))
}

func ClearAuthCookie(w http.ResponseWriter, serverMode myncer_pb.ServerMode) {
	http.SetCookie(w, GetLogoutAuthCookie(serverMode))
}
