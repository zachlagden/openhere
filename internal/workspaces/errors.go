package workspaces

import "errors"

var (
	ErrNotFound          = errors.New("not found")
	ErrForbidden         = errors.New("the role does not allow this action")
	ErrSlugTaken         = errors.New("slug is already taken")
	ErrLastAdmin         = errors.New("a shared workspace needs at least one workspace-wide admin")
	ErrPersonalWorkspace = errors.New("personal workspaces cannot be shared, deleted or have their members changed")
	ErrNotEmpty          = errors.New("workspace still contains sites")
	ErrMemberExists      = errors.New("user already has this access")
	ErrUserNotFound      = errors.New("no user has that email")
)
