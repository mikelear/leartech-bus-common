// Package maestro exposes a typed Go client for the leartech-maestro-service
// event broker (PRODUCE — via the oapi-codegen-generated Client) alongside a
// gin HTTP handler for the CONSUME callback that Maestro POSTs to
// subscribing services (durable, retried).
//
// # Files in this package
//
//   - swagger.yaml           - upstream Swagger 2.0 spec vendored verbatim from
//     leartech-maestro-service/docs/swagger.yaml. Ground-truth reference.
//   - openapi.yaml           - OpenAPI 3.0 transcription of swagger.yaml,
//     narrowed to the endpoints Go services consume today. oapi-codegen v2
//     only reads OAS3, so we vendor both files: swagger.yaml as the humans'
//     source of truth, openapi.yaml as the generator's input.
//   - oapi-codegen-config.yaml - generator config (models + client with responses).
//   - client_generated.go    - GENERATED. Do NOT edit by hand.
//   - handler.go             - hand-written gin consume-side dispatcher.
//   - producer.go            - hand-written thin wrapper around the generated
//     client that plugs the caller-provided *http.Client (used to attach the
//     bus-common s2s bearer via LEARTECH_AUTH_*).
//
// # Regenerating the client
//
// The vendored swagger.yaml is the source of truth. When Maestro's API
// changes:
//
//  1. Refresh swagger.yaml:
//     gh api repos/mikelear/leartech-maestro-service/contents/docs/swagger.yaml?ref=main \
//     --jq '.content' | base64 -d > pkg/maestro/swagger.yaml
//  2. Transcribe the delta by hand into openapi.yaml (only endpoints Go
//     consumes need transcribing — see the file's header for scope).
//  3. Run:
//     go generate ./pkg/maestro/...
//  4. Commit swagger.yaml, openapi.yaml, and the regenerated
//     client_generated.go together.
//
// The //go:generate directive below assumes `oapi-codegen` (v2) is on PATH.
// Install with:
//
//	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
//
//go:generate oapi-codegen -config oapi-codegen-config.yaml openapi.yaml
package maestro
