package service

import (
	"fmt"
	"time"

	"github.com/Hiroki111/go-carshop-backend/user-service/internal/auth"
)

// ServiceToken is an access token issued to another service.
type ServiceToken struct {
	AccessToken string
	ExpiresIn   time.Duration
	Scopes      []string
}

// IssueServiceToken implements the client-credentials grant: it authenticates
// the calling service and, if that works, signs a token with the scopes it asked
// for (or all it is allowed, when requestedScope is empty).
//
// It returns oauth.ErrInvalidClient when the client ID or secret is wrong and
// oauth.ErrInvalidScope when a requested scope isn't allowed for the client.
func (s *Service) IssueServiceToken(clientID, clientSecret, requestedScope string) (ServiceToken, error) {
	client, err := s.clients.Authenticate(clientID, clientSecret)
	if err != nil {
		return ServiceToken{}, err
	}

	scopes, err := client.ResolveScopes(requestedScope)
	if err != nil {
		return ServiceToken{}, err
	}

	accessToken, err := auth.GenerateServiceToken(s.privateKey, client.ID, scopes)
	if err != nil {
		return ServiceToken{}, fmt.Errorf("failed to sign the service token: %w", err)
	}

	return ServiceToken{
		AccessToken: accessToken,
		ExpiresIn:   auth.ServiceTokenTTL,
		Scopes:      scopes,
	}, nil
}
