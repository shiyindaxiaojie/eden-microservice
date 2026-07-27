// Package compat exposes the stable Consul wire-shape helpers used by transports.
package compat

import internal "eden-microservice/apps/registry/internal/adapter/consul/compat"

type (
	CatalogServiceEnvelope = internal.CatalogServiceEnvelope
	Deregistration         = internal.Deregistration
	Instance               = internal.Instance
)

var (
	ApplyHeaders                 = internal.ApplyHeaders
	BuildCatalogServiceEnvelopes = internal.BuildCatalogServiceEnvelopes
	BuildHealthServiceEntries    = internal.BuildHealthServiceEntries
	BuildServicesMap             = internal.BuildServicesMap
	DecodeCatalogInstances       = internal.DecodeCatalogInstances
	DecodeDeregisterRequest      = internal.DecodeDeregisterRequest
	DecodeRegisterRequest        = internal.DecodeRegisterRequest
	DecodeServicesMap            = internal.DecodeServicesMap
	PublicMetadata               = internal.PublicMetadata
	StoredCheckID                = internal.StoredCheckID
	StoredTags                   = internal.StoredTags
)
