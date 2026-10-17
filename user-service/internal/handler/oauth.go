package handler

import (
	"errors"
	"log"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/Hiroki111/go-carshop-backend/user-service/internal/oauth"
)

const (
	grantTypeClientCredentials = "client_credentials"

	// A token request is a handful of short parameters.
	maxTokenRequestBytes = 4 << 10
)

// Error codes of RFC 6749 section 5.2.
const (
	oauthInvalidRequest       = "invalid_request"
	oauthInvalidClient        = "invalid_client"
	oauthInvalidScope         = "invalid_scope"
	oauthUnsupportedGrantType = "unsupported_grant_type"
	oauthServerError          = "server_error"
)

var (
	errNoClientCredentials        = errors.New("client authentication is required")
	errMalformedClientCredentials = errors.New("malformed client credentials")
	errTwoClientAuthMethods       = errors.New("use only one client authentication method")
)

// IssueToken godoc
// @Summary      Issue a service token
// @Description  OAuth2 token endpoint for the client-credentials grant (RFC 6749 section 4.4). A registered service (for example order-service) authenticates with its client ID and secret and receives a short-lived access token that carries scopes such as cars:reserve. The client authenticates with HTTP Basic (preferred) or with client_id and client_secret in the body, not both. Without a scope parameter the token gets every scope the client is allowed.
// @Tags         auth
// @Accept       x-www-form-urlencoded
// @Produce      json
// @Security     BasicAuth
// @Param        grant_type     formData  string  true   "Must be client_credentials"
// @Param        scope          formData  string  false  "Space-separated scopes to request"
// @Param        client_id      formData  string  false  "Client ID, when not using HTTP Basic"
// @Param        client_secret  formData  string  false  "Client secret, when not using HTTP Basic"
// @Success      200            {object}  TokenResponse
// @Failure      400            {object}  OAuthErrorResponse
// @Failure      401            {object}  OAuthErrorResponse
// @Failure      500            {object}  OAuthErrorResponse
// @Router       /oauth/token [post]
func (h *Handler) IssueToken(w http.ResponseWriter, r *http.Request) {
	// The token response must never be cached, errors included (RFC 6749 section 5.1).
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")

	r.Body = http.MaxBytesReader(w, r.Body, maxTokenRequestBytes)

	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/x-www-form-urlencoded" {
		writeOAuthError(w, http.StatusBadRequest, oauthInvalidRequest, "Content-Type must be application/x-www-form-urlencoded")
		return
	}
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, oauthInvalidRequest, "malformed request body")
		return
	}

	form := r.PostForm

	for _, name := range []string{"grant_type", "scope", "client_id", "client_secret"} {
		if len(form[name]) > 1 {
			writeOAuthError(w, http.StatusBadRequest, oauthInvalidRequest, "parameter "+name+" must not be repeated")
			return
		}
	}

	grantType := form.Get("grant_type")
	if grantType == "" {
		writeOAuthError(w, http.StatusBadRequest, oauthInvalidRequest, "grant_type is required")
		return
	}
	if grantType != grantTypeClientCredentials {
		writeOAuthError(w, http.StatusBadRequest, oauthUnsupportedGrantType, "only the client_credentials grant is supported")
		return
	}

	clientID, clientSecret, err := clientCredentials(r, form)
	if err != nil {
		if errors.Is(err, errTwoClientAuthMethods) {
			writeOAuthError(w, http.StatusBadRequest, oauthInvalidRequest, err.Error())
			return
		}
		writeInvalidClient(w, err.Error())
		return
	}

	token, err := h.service.IssueServiceToken(clientID, clientSecret, form.Get("scope"))
	if err != nil {
		switch {
		case errors.Is(err, oauth.ErrInvalidClient):
			log.Printf("oauth: client authentication failed for client_id %q", clientID)
			writeInvalidClient(w, "client authentication failed")
		case errors.Is(err, oauth.ErrInvalidScope):
			writeOAuthError(w, http.StatusBadRequest, oauthInvalidScope, err.Error())
		default:
			log.Printf("oauth: failed to issue a token: %v", err)
			writeOAuthError(w, http.StatusInternalServerError, oauthServerError, "failed to issue a token")
		}
		return
	}

	writeJSON(w, http.StatusOK, TokenResponse{
		AccessToken: token.AccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int(token.ExpiresIn.Seconds()),
		Scope:       strings.Join(token.Scopes, " "),
	})
}

// clientCredentials reads the client's ID and secret from either an HTTP Basic
// Authorization header or the form body. A request may use only one of the two
// (RFC 6749 section 2.3).
func clientCredentials(r *http.Request, form url.Values) (id, secret string, err error) {
	header := r.Header.Get("Authorization")
	hasBasic := len(header) >= len("Basic ") && strings.EqualFold(header[:len("Basic ")], "Basic ")
	hasBody := form.Has("client_id") || form.Has("client_secret")

	switch {
	case hasBasic && hasBody:
		return "", "", errTwoClientAuthMethods

	case hasBasic:
		basicID, basicSecret, ok := r.BasicAuth()
		if !ok {
			return "", "", errMalformedClientCredentials
		}
		// RFC 6749 section 2.3.1: both parts are form-urlencoded before they are
		// base64-encoded into the header, so they must be decoded again.
		decodedID, idErr := url.QueryUnescape(basicID)
		decodedSecret, secretErr := url.QueryUnescape(basicSecret)
		if idErr != nil || secretErr != nil {
			return "", "", errMalformedClientCredentials
		}
		return decodedID, decodedSecret, nil

	case hasBody:
		return form.Get("client_id"), form.Get("client_secret"), nil

	default:
		return "", "", errNoClientCredentials
	}
}

func writeInvalidClient(w http.ResponseWriter, description string) {
	// RFC 6749 section 5.2: a 401 must say which authentication scheme to use.
	w.Header().Set("WWW-Authenticate", `Basic realm="user-service", charset="UTF-8"`)
	writeOAuthError(w, http.StatusUnauthorized, oauthInvalidClient, description)
}

func writeOAuthError(w http.ResponseWriter, status int, code, description string) {
	writeJSON(w, status, OAuthErrorResponse{Error: code, ErrorDescription: description})
}
