package main

import (
	"context"
	"net/http"

	"github.com/ogen-go/ogen/ogenerrors"

	fleetv1 "github.com/umartahir93/olivaw-assignment/gen/fleet/v1"
)

// problemErrorHandler writes the errors the generated server detects itself,
// such as an invalid parameter or body, as problem details (RFC 9457), like
// every other error in the contract.
func problemErrorHandler(_ context.Context, w http.ResponseWriter, _ *http.Request, err error) {
	status := ogenerrors.ErrorCode(err)
	problem := fleetv1.Problem{
		Title:  http.StatusText(status),
		Status: status,
		Detail: fleetv1.NewOptString(err.Error()),
	}
	body, _ := problem.MarshalJSON()
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
