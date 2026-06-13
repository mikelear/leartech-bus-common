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
}
