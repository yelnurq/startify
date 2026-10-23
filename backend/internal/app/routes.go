package app

import (
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/platform/httpx"
)

// apiBase is where the operations of openapi.yaml are served.
const apiBase = "/api/v1"

// specRoutes reads from openapi.yaml the operations that need more room than
// the rest: event streams (no request timeout) and file uploads (the larger
// body limit). Reading them from the contract means a new operation cannot
// be forgotten in a hand-written list.
func specRoutes() (streams, uploads httpx.Routes, err error) {
	spec, err := apigen.GetSwagger()
	if err != nil {
		return httpx.Routes{}, httpx.Routes{}, fmt.Errorf("app: read openapi.yaml: %w", err)
	}
	var streamList, uploadList []string
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			route := method + " " + apiBase + path
			if op.RequestBody != nil && op.RequestBody.Value != nil && op.RequestBody.Value.Content["multipart/form-data"] != nil {
				uploadList = append(uploadList, route)
			}
			if streamsEvents(op) {
				streamList = append(streamList, route)
			}
		}
	}
	return httpx.NewRoutes(streamList...), httpx.NewRoutes(uploadList...), nil
}

// streamsEvents reports whether a response of the operation is an event stream.
func streamsEvents(op *openapi3.Operation) bool {
	if op.Responses == nil {
		return false
	}
	for _, resp := range op.Responses.Map() {
		if resp.Value != nil && resp.Value.Content["text/event-stream"] != nil {
			return true
		}
	}
	return false
}
