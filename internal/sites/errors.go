package sites

import "errors"

var (
	ErrSlugTaken       = errors.New("slug is already taken")
	ErrFolderNameTaken = errors.New("a folder with that name already exists in the workspace")
	ErrFolderHasGrants = errors.New("the folder still has folder-scoped access grants")
	ErrSlugGeneration  = errors.New("could not generate an unused slug")
)
