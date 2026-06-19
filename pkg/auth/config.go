package auth

// Config holds the configuration for the auth package, which is used to authenticate with an authorisation server and perform endpoint authentication middleware.
// The configuration can be set using environment variables or a YAML config file. The environment variable names are specified in the struct tags.
// See [mqube-go-service-barebones] for an example of how to use this config in a service.
//
// For deploying MQube services in Kubernetes Config.ServerURL, Config.ClientID, and Config.ClientSecret should all come from environment variables that are set via Kubernetes secrets.
// Deployment.yaml (on the service container) example:
//
//	env:
//	- name: MQUBE_AUTH_SERVER_URL
//	  valueFrom:
//	    secretKeyRef:
//	      key: BASE_URL
//	      name: backend-service-oauth
//	- name: MQUBE_AUTH_CLIENT_ID
//	  valueFrom:
//	    secretKeyRef:
//	      key: CLIENT_ID
//	      name: backend-service-oauth
//	- name: MQUBE_AUTH_CLIENT_SECRET
//	  valueFrom:
//	    secretKeyRef:
//	      key: CLIENT_SECRET
//	      name: backend-service-oauth
//
// For local development, these can be set in the development-config.yaml file or as environment variables.
// Example auth section in development-config.yaml:
//
//	auth:
//	  serverURL: https://hydra-jx-staging.jx.mqube.build
//	  clientID: my-client-id
//	  clientSecret: my-client-secret
//
// [mqube-go-service-barebones]: https://github.com/spring-financial-group/mqube-go-service-barebones
type Config struct {
	// ServerURL is the URL of the authorisation server (e.g. https://hydra-jx-staging.jx.mqube.build)
	ServerURL string `env:"MQUBE_AUTH_SERVER_URL" yaml:"serverURL"`
	// ClientID is the ID of the client that the service will use to authenticate with the authorisation server
	ClientID string `env:"MQUBE_AUTH_CLIENT_ID" yaml:"clientID"`
	// ClientSecret is the secret for the client that the service will use to authenticate with the authorisation server
	ClientSecret string `env:"MQUBE_AUTH_CLIENT_SECRET" yaml:"clientSecret"`
	// DisableMiddleware stops the endpoint authentication middleware from being performed.
	// Useful for local development. Do not use in production.
	DisableMiddleware bool `yaml:"disableMiddleware"`

	// Resource is the canonical URI of this resource server (RFC 9728 §3.1).
	// Optional. When set together with AuthorizationServers, the package can
	// serve a protected-resource-metadata document via ResourceMetadataHandler.
	// Empty default = discovery feature disabled (backward-compatible).
	Resource string `env:"MQUBE_AUTH_RESOURCE" yaml:"resource"`

	// AuthorizationServers is the list of authorisation server issuer URLs that
	// can issue tokens for this resource (RFC 9728 §3.1).
	// Optional. Empty default = discovery feature disabled (backward-compatible).
	AuthorizationServers []string `env:"MQUBE_AUTH_AUTHORIZATION_SERVERS" yaml:"authorizationServers"`

	// ResourceMetadataURL is the absolute URL where the protected-resource-metadata
	// document is published. When set, the auth middleware adds a
	// `WWW-Authenticate: Bearer resource_metadata="<url>"` header to its 401
	// responses (RFC 9728 §5.1).
	// Optional. Empty default = WWW-Authenticate hint disabled (backward-compatible
	// with existing consumers' 401s).
	ResourceMetadataURL string `env:"MQUBE_AUTH_RESOURCE_METADATA_URL" yaml:"resourceMetadataURL"`
}
