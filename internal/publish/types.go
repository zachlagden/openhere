package publish

import "time"

const (
	StatusPending   = "pending"
	StatusFinalized = "finalized"
)

type FileSpec struct {
	Path        string
	Size        int64
	SHA256      string
	ContentType string
}

type Version struct {
	ID            string
	SiteID        string
	Status        string
	CreatedBy     string
	CreatedAt     time.Time
	FinalizedAt   *time.Time
	FileCount     int
	TotalBytes    int64
	BaseVersionID string
	Live          bool
}

func (v Version) ExpiresAt() time.Time {
	return v.CreatedAt.Add(PendingTTL)
}

type Manifest struct {
	VersionID string
	Files     []FileSpec
}
