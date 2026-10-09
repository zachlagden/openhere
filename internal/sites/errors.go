package sites

import "errors"

var (
	ErrSlugTaken       = errors.New("slug is already taken")
	ErrFolderNameTaken = errors.New("a folder with that name already exists in the workspace")
	ErrSlugGeneration  = errors.New("could not generate an unused slug")
)
