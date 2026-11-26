package handlers

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
)

// statusClientClosedRequest is the nginx convention for a request whose client
// disconnected before the server answered. net/http has no constant for it.
const statusClientClosedRequest = 499

// AbortIfCanceled answers a request whose client already went away. A canceled
// context is not a server failure: returning 500 or 404 for it invents an error
// nobody reads (the portal retries and usually succeeds), and it drowns real
// incidents in the logs. Handlers call this first in their error branch and
// return when it reports true.
//
// The client cannot read a body, so the status exists only to record the
// outcome in the request log.
func AbortIfCanceled(c *gin.Context, err error) bool {
	if !errors.Is(err, context.Canceled) {
		return false
	}
	c.Status(statusClientClosedRequest)
	return true
}
