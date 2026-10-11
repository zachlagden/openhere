package api

import (
	"net/url"
	"strings"
	"time"

	"github.com/zachlagden/openhere/internal/hosting"
	"github.com/zachlagden/openhere/internal/publish"
)

type fileSpecRequest struct {
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	ContentType string `json:"contentType"`
}

type publishRequest struct {
	Workspace   string            `json:"workspace"`
	Slug        string            `json:"slug"`
	Folder      string            `json:"folder"`
	DisplayName string            `json:"displayName"`
	Description string            `json:"description"`
	SpaMode     bool              `json:"spaMode"`
	Files       []fileSpecRequest `json:"files"`
}

type republishRequest struct {
	BaseVersionID string            `json:"baseVersionId"`
	Files         []fileSpecRequest `json:"files"`
}

func toFileSpecs(files []fileSpecRequest) []publish.FileSpec {
	specs := make([]publish.FileSpec, len(files))
	for i, f := range files {
		specs[i] = publish.FileSpec{Path: f.Path, Size: f.Size, SHA256: f.SHA256, ContentType: f.ContentType}
	}
	return specs
}

type uploadResponse struct {
	Path    string            `json:"path"`
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

type publishResponse struct {
	Slug        string              `json:"slug"`
	SiteID      string              `json:"siteId"`
	VersionID   string              `json:"versionId"`
	ExpiresAt   time.Time           `json:"expiresAt"`
	Uploads     []uploadResponse    `json:"uploads"`
	FinalizeURL string              `json:"finalizeUrl"`
	URL         *string             `json:"url"`
	Hosting     siteHostingResponse `json:"hosting"`
}

func toPublishResponse(p publish.Pending, availability hosting.SiteAvailability) publishResponse {
	versionURL := basePath + "/publish/" + p.Site.Slug + "/versions/" + p.Version.ID
	uploads := make([]uploadResponse, len(p.Files))
	for i, f := range p.Files {
		uploads[i] = uploadResponse{
			Path: f.Path, Method: "PUT", URL: versionURL + "/files/" + escapePath(f.Path),
			Headers: map[string]string{"Content-Type": "application/octet-stream"},
		}
	}
	return publishResponse{
		Slug: p.Site.Slug, SiteID: p.Site.ID, VersionID: p.Version.ID, ExpiresAt: p.Version.ExpiresAt(),
		Uploads: uploads, FinalizeURL: versionURL + "/finalize", URL: optional(availability.URL), Hosting: toSiteHostingResponse(availability),
	}
}

func escapePath(p string) string {
	segments := strings.Split(p, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}

type versionResponse struct {
	ID            string     `json:"id"`
	SiteID        string     `json:"siteId"`
	Status        string     `json:"status"`
	Live          bool       `json:"live"`
	CreatedBy     *string    `json:"createdBy"`
	CreatedAt     time.Time  `json:"createdAt"`
	FinalizedAt   *time.Time `json:"finalizedAt"`
	FileCount     int        `json:"fileCount"`
	TotalBytes    int64      `json:"totalBytes"`
	BaseVersionID *string    `json:"baseVersionId"`
}

func toVersionResponse(v publish.Version) versionResponse {
	return versionResponse{
		ID: v.ID, SiteID: v.SiteID, Status: v.Status, Live: v.Live, CreatedBy: optional(v.CreatedBy), CreatedAt: v.CreatedAt,
		FinalizedAt: v.FinalizedAt, FileCount: v.FileCount, TotalBytes: v.TotalBytes, BaseVersionID: optional(v.BaseVersionID),
	}
}

type versionListResponse struct {
	Versions []versionResponse `json:"versions"`
}

type resultResponse struct {
	Slug    string              `json:"slug"`
	URL     *string             `json:"url"`
	Hosting siteHostingResponse `json:"hosting"`
	Version versionResponse     `json:"version"`
}

func toResultResponse(r publish.Result, availability hosting.SiteAvailability) resultResponse {
	return resultResponse{
		Slug: r.Site.Slug, URL: optional(availability.URL), Hosting: toSiteHostingResponse(availability), Version: toVersionResponse(r.Version),
	}
}

type fileResponse struct {
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	ContentType string `json:"contentType"`
}

type manifestResponse struct {
	VersionID *string        `json:"versionId"`
	Files     []fileResponse `json:"files"`
}

func toFileResponse(f publish.FileSpec) fileResponse {
	return fileResponse{Path: f.Path, Size: f.Size, SHA256: f.SHA256, ContentType: f.ContentType}
}

type fileEnvelope struct {
	File fileResponse `json:"file"`
}
