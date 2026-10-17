// Package oauth holds the OAuth2 clients (other services) that may ask
// user-service for an access token with the client-credentials grant
// (RFC 6749 section 4.4), and the scopes each of them may be granted.
package oauth

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Scopes a service token can carry. car-service checks them, so the names are
// part of the contract between the two services.
const (
	ScopeCarsReserve = "cars:reserve"
	ScopeCarsRelease = "cars:release"
	ScopeCarsSold    = "cars:sold"
)

// MinSecretLength is the shortest client secret we accept. Generate one with
// `openssl rand -hex 32` (64 characters).
const MinSecretLength = 32

var (
	ErrInvalidClient = errors.New("invalid client")
	ErrInvalidScope  = errors.New("invalid scope")
)

// Client is a service that is allowed to request tokens.
type Client struct {
	ID            string
	secretDigest  [sha256.Size]byte
	allowedScopes []string
}

// NewClient registers a client. Only a SHA-256 digest of the secret is kept.
//
// A fast hash is fine here, unlike for user passwords: the secret is a long
// random string, so there is nothing to guess and slowing down brute force
// (what bcrypt is for) adds nothing. It also avoids bcrypt's 72-byte input limit
// and its cost on every token request.
func NewClient(id, secret string, allowedScopes []string) (Client, error) {
	if id == "" {
		return Client{}, errors.New("oauth: client ID must not be empty")
	}
	if len(secret) < MinSecretLength {
		return Client{}, fmt.Errorf("oauth: the secret of client %q must be at least %d characters (generate one with: openssl rand -hex 32)", id, MinSecretLength)
	}
	if len(allowedScopes) == 0 {
		return Client{}, fmt.Errorf("oauth: client %q needs at least one allowed scope", id)
	}
	for _, scope := range allowedScopes {
		if scope == "" || strings.ContainsAny(scope, " \t\r\n") {
			return Client{}, fmt.Errorf("oauth: client %q has an invalid scope %q", id, scope)
		}
	}

	return Client{
		ID:            id,
		secretDigest:  sha256.Sum256([]byte(secret)),
		allowedScopes: slices.Clone(allowedScopes),
	}, nil
}

// AllowedScopes returns a copy of the scopes this client may be granted.
func (c Client) AllowedScopes() []string {
	return slices.Clone(c.allowedScopes)
}

// ResolveScopes turns the space-separated scope parameter of a token request
// into the scopes to grant. An empty request means every scope the client is
// allowed. Asking for a scope the client isn't allowed fails with
// ErrInvalidScope: we don't silently hand out a smaller set than was asked for.
func (c Client) ResolveScopes(requested string) ([]string, error) {
	fields := strings.Fields(requested)
	if len(fields) == 0 {
		return c.AllowedScopes(), nil
	}

	granted := make([]string, 0, len(fields))
	for _, scope := range fields {
		if !slices.Contains(c.allowedScopes, scope) {
			return nil, fmt.Errorf("%w: %q is not allowed for this client", ErrInvalidScope, scope)
		}
		if !slices.Contains(granted, scope) {
			granted = append(granted, scope)
		}
	}

	return granted, nil
}

// Registry is the set of known clients. It is built at startup and then only
// read, so it needs no locking.
type Registry struct {
	clients map[string]Client
}

func NewRegistry(clients ...Client) (*Registry, error) {
	registry := &Registry{clients: make(map[string]Client, len(clients))}

	for _, client := range clients {
		if _, exists := registry.clients[client.ID]; exists {
			return nil, fmt.Errorf("oauth: client %q is registered twice", client.ID)
		}
		registry.clients[client.ID] = client
	}

	return registry, nil
}

// Authenticate returns the client when clientID exists and secret is its
// secret, and ErrInvalidClient otherwise. The digests are always compared, even
// for an unknown client, and in constant time, so the response doesn't reveal
// whether the ID or the secret was wrong.
func (r *Registry) Authenticate(clientID, secret string) (Client, error) {
	var client Client
	var found bool
	if r != nil {
		client, found = r.clients[clientID]
	}

	given := sha256.Sum256([]byte(secret))
	matches := subtle.ConstantTimeCompare(given[:], client.secretDigest[:]) == 1

	if !found || !matches {
		return Client{}, ErrInvalidClient
	}

	return client, nil
}
