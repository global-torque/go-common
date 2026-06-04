//nolint:gochecknoglobals
package response

var (
	// MsgBadRequest used to indicate error in incoming data
	MsgBadRequest = SingleErrorMessage("Bad request.")
	// MsgNotFound typically used when element haven't been found
	MsgNotFound = SingleErrorMessage("Not found.")
	// MsgUnauthorized signalizes lack of token or other authorization data
	MsgUnauthorized = SingleErrorMessage("Authorization error.")
	// MsgForbidden used when user does not have any permissions to perform action
	MsgForbidden = SingleErrorMessage("Forbidden error.")
	// MsgInternalErr server side error
	MsgInternalErr = SingleErrorMessage("Internal/server error.")
)
