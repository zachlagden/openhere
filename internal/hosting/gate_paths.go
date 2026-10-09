package hosting

import "strings"

const (
	gateBase     = "/.well-known/selfhost-gate/"
	UnlockPath   = gateBase + "unlock"
	CodePath     = gateBase + "code"
	VerifyPath   = gateBase + "verify"
	CallbackPath = "/.well-known/selfhost-auth"
	HandoffPath  = "/api/v1/access/handoff"
)

func isGatePost(requestPath string) bool {
	return strings.HasPrefix(requestPath, gateBase)
}
