package tracker

import "fmt"

// ErrNotFound is the sentinel that callers should match with errors.Is when
// they need to distinguish "resource does not exist" from transient failures.
// Both the Linear and GitHub adapters wrap this via NotFoundError.
var ErrNotFound = fmt.Errorf("tracker: resource not found")

// APIStatusError is returned when a tracker HTTP API responds with a
// non-2xx status code. Use errors.As to access the specific status:
//
//	var apiErr *tracker.APIStatusError
//	if errors.As(err, &apiErr) {
//	    log.Printf("tracker returned HTTP %d", apiErr.Status)
//	}
type APIStatusError struct {
	// Adapter identifies the backend ("linear" or "github").
	Adapter string
	// Status is the HTTP response status code (e.g. 401, 429, 500).
	Status int
}

func (e *APIStatusError) Error() string {
	return fmt.Sprintf("%s: api status %d", e.Adapter, e.Status)
}

// NotFoundError is returned when a requested resource does not exist in the
// tracker (HTTP 404, deleted issue, or missing issue identifier).
// It satisfies errors.Is(err, tracker.ErrNotFound).
//
//	var nfe *tracker.NotFoundError
//	if errors.Is(err, tracker.ErrNotFound) { ... }           // sentinel check
//	if errors.As(err, &nfe) { log.Print(nfe.Identifier) }   // detail access
type NotFoundError struct {
	// Adapter identifies the backend ("linear" or "github").
	Adapter string
	// Identifier is the issue identifier, if known at the call site.
	Identifier string
}

func (e *NotFoundError) Error() string {
	if e.Identifier != "" {
		return fmt.Sprintf("%s: %s not found", e.Adapter, e.Identifier)
	}
	return fmt.Sprintf("%s: resource not found", e.Adapter)
}

// Is reports whether this error matches the ErrNotFound sentinel, enabling
// errors.Is(err, tracker.ErrNotFound) across the call chain.
func (e *NotFoundError) Is(target error) bool {
	return target == ErrNotFound
}

// GraphQLError is returned by the Linear adapter when the GraphQL response
// body contains an "errors" field. This typically indicates an authentication
// failure, a malformed query, or a Linear API schema change.
//
//	var gqlErr *tracker.GraphQLError
//	if errors.As(err, &gqlErr) {
//	    log.Printf("linear graphql error: %s", gqlErr.Message)
//	}
type GraphQLError struct {
	// Message is the raw string representation of the GraphQL errors array.
	Message string
}

func (e *GraphQLError) Error() string {
	return fmt.Sprintf("linear: graphql error: %s", e.Message)
}
