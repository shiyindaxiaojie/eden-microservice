// Package api defines the public permission-control contract.
package auth

import internal "eden-microservice/apps/auth/internal/auth"

type (
	APIKey        = internal.APIKey
	Authenticator = internal.Authenticator
	Store         = internal.Store
	User          = internal.User
)

var (
	NewAuthenticator = internal.NewAuthenticator
	NewStore         = internal.NewStore
)
