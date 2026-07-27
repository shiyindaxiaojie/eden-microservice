// Package module is the public composition surface for permission control.
package module

import api "eden-microservice/apps/auth/api"

type Container struct {
	Store         *api.Store
	Authenticator api.Authenticator
}

func NewContainer(dataPath string) *Container {
	store := api.NewStore(dataPath)
	return &Container{Store: store, Authenticator: api.NewAuthenticator(store)}
}
