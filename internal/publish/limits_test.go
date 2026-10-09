package publish

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/zachlagden/selfhost/internal/sitepath"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
	"github.com/zachlagden/selfhost/internal/validate"
)

func TestManifestLimits(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		e.set(`{"limits.maxFilesPerVersion": 3, "limits.maxFileBytes": 10, "limits.maxVersionBytes": 25}`)
		begin := func(files site) error {
			_, err := e.svc.Begin(ctx, BeginInput{UserID: e.alex, Workspace: e.acme.Slug, Files: specs(files)})
			return err
		}
		if err := begin(site{"a": "1", "b": "2", "c": "3", "d": "4"}); !errors.Is(err, ErrTooManyFiles) {
			t.Errorf("four files = %v, want ErrTooManyFiles", err)
		}
		if err := begin(site{"a": strings.Repeat("x", 11)}); !errors.Is(err, ErrFileTooLarge) {
			t.Errorf("11 byte file = %v, want ErrFileTooLarge", err)
		}
		if err := begin(site{"a": strings.Repeat("x", 10), "b": strings.Repeat("x", 10), "c": strings.Repeat("x", 6)}); !errors.Is(err, ErrVersionTooLarge) {
			t.Errorf("26 byte version = %v, want ErrVersionTooLarge", err)
		}
		if err := begin(site{"a": strings.Repeat("x", 10), "b": strings.Repeat("x", 10), "c": strings.Repeat("x", 5)}); err != nil {
			t.Errorf("limits are inclusive: %v", err)
		}
	})
}

func TestLimitsApplyToNewVersionsToo(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		first := e.publishNew("limited", site{"index.html": "ok"})
		e.set(`{"limits.maxFilesPerVersion": 1}`)
		_, err := e.svc.NewVersion(context.Background(), NewVersionInput{
			UserID: e.alex, Site: "limited", BaseVersionID: first.Version.ID, Files: specs(site{"a": "1", "b": "2"}),
		})
		if !errors.Is(err, ErrTooManyFiles) {
			t.Errorf("error = %v, want ErrTooManyFiles", err)
		}
	})
}

func TestFinalizeAndUploadRecheckLoweredLimits(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		files := site{"a.txt": "12345678", "b.txt": "12345678"}
		p := e.begin("lowered", files)
		e.upload(e.alex, p, files)

		e.set(`{"limits.maxFilesPerVersion": 1}`)
		if _, err := e.svc.Finalize(ctx, e.alex, "lowered", p.Version.ID); !errors.Is(err, ErrTooManyFiles) {
			t.Errorf("file count = %v, want ErrTooManyFiles", err)
		}
		e.set(`{"limits.maxFilesPerVersion": 10, "limits.maxVersionBytes": 10}`)
		if _, err := e.svc.Finalize(ctx, e.alex, "lowered", p.Version.ID); !errors.Is(err, ErrVersionTooLarge) {
			t.Errorf("total size = %v, want ErrVersionTooLarge", err)
		}
		e.set(`{"limits.maxVersionBytes": 1000, "limits.maxFileBytes": 4}`)
		if _, err := e.svc.Finalize(ctx, e.alex, "lowered", p.Version.ID); !errors.Is(err, ErrFileTooLarge) {
			t.Errorf("file size = %v, want ErrFileTooLarge", err)
		}
		if _, err := e.svc.Upload(ctx, e.alex, "lowered", p.Version.ID, "a.txt", strings.NewReader("12345678")); !errors.Is(err, ErrFileTooLarge) {
			t.Errorf("upload = %v, want ErrFileTooLarge", err)
		}
		e.set(`{"limits.maxFileBytes": 100}`)
		if _, err := e.svc.Finalize(ctx, e.alex, "lowered", p.Version.ID); err != nil {
			t.Errorf("Finalize with limits restored: %v", err)
		}
	})
}

func TestManifestValidation(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		good := FileSpec{Path: "index.html", Size: 1, SHA256: hashOf("x")}
		with := func(change func(*FileSpec)) []FileSpec {
			spec := good
			change(&spec)
			return []FileSpec{spec}
		}
		cases := map[string][]FileSpec{
			"no files":          nil,
			"negative size":     with(func(f *FileSpec) { f.Size = -1 }),
			"short hash":        with(func(f *FileSpec) { f.SHA256 = "abc" }),
			"non hex hash":      with(func(f *FileSpec) { f.SHA256 = strings.Repeat("z", 64) }),
			"empty hash":        with(func(f *FileSpec) { f.SHA256 = "" }),
			"content type junk": with(func(f *FileSpec) { f.ContentType = "not a media type" }),
			"content type crlf": with(func(f *FileSpec) { f.ContentType = "text/html\r\nX-Evil: 1" }),
			"content type long": with(func(f *FileSpec) { f.ContentType = "text/" + strings.Repeat("a", 300) }),
		}
		for name, files := range cases {
			_, err := e.svc.Begin(ctx, BeginInput{UserID: e.alex, Workspace: e.acme.Slug, Files: files})
			var invalid *validate.Error
			if !errors.As(err, &invalid) {
				t.Errorf("%s: error = %v, want a validation error", name, err)
			}
		}
	})
}

func TestManifestNormalisesHashAndContentType(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		p, err := e.svc.Begin(context.Background(), BeginInput{UserID: e.alex, Workspace: e.acme.Slug, Files: []FileSpec{
			{Path: "page.html", Size: 1, SHA256: strings.ToUpper(hashOf("x"))},
			{Path: "data.bin.unknownext", Size: 1, SHA256: hashOf("y")},
			{Path: "styled.css", Size: 1, SHA256: hashOf("z"), ContentType: "text/css; charset=utf-8"},
		}})
		if err != nil {
			t.Fatal(err)
		}
		byPath := map[string]FileSpec{}
		for _, f := range p.Files {
			byPath[f.Path] = f
		}
		if byPath["page.html"].SHA256 != hashOf("x") {
			t.Errorf("hash not lower-cased: %s", byPath["page.html"].SHA256)
		}
		if !strings.HasPrefix(byPath["page.html"].ContentType, "text/html") {
			t.Errorf("html content type = %q", byPath["page.html"].ContentType)
		}
		if byPath["data.bin.unknownext"].ContentType != "application/octet-stream" {
			t.Errorf("fallback content type = %q", byPath["data.bin.unknownext"].ContentType)
		}
		if byPath["styled.css"].ContentType != "text/css; charset=utf-8" {
			t.Errorf("explicit content type = %q", byPath["styled.css"].ContentType)
		}
	})
}

func TestManifestPathSafety(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		bad := map[string][]string{
			"empty path":          {""},
			"absolute":            {"/etc/passwd"},
			"dot segment":         {"a/./b"},
			"dotdot segment":      {"../outside"},
			"nested dotdot":       {"a/b/../../../c"},
			"empty segment":       {"a//b"},
			"trailing slash":      {"dir/"},
			"backslash":           {`a\b.txt`},
			"windows traversal":   {`..\..\boot.ini`},
			"nul":                 {"a\x00.txt"},
			"control":             {"a\x01b"},
			"newline":             {"a\nb"},
			"too long":            {strings.Repeat("a/", 600)},
			"long segment":        {strings.Repeat("a", 256)},
			"duplicate":           {"a.html", "a.html"},
			"case collision":      {"README.md", "readme.md"},
			"case collision deep": {"Docs/Guide.html", "docs/guide.HTML"},
			"file and dir":        {"docs", "docs/index.html"},
		}
		for name, paths := range bad {
			files := make([]FileSpec, len(paths))
			for i, p := range paths {
				files[i] = FileSpec{Path: p, Size: 1, SHA256: hashOf("x")}
			}
			_, err := e.svc.Begin(ctx, BeginInput{UserID: e.alex, Workspace: e.acme.Slug, Files: files})
			if !errors.Is(err, sitepath.ErrInvalid) {
				t.Errorf("%s: error = %v, want ErrInvalid", name, err)
			}
			if _, err := e.svc.NewVersion(ctx, NewVersionInput{UserID: e.alex, Site: "no-such-site", Files: files}); err == nil {
				t.Errorf("%s: NewVersion accepted the manifest", name)
			}
		}
		if _, err := e.sites.ListSites(ctx, e.alex, listAll); err != nil {
			t.Fatal(err)
		}
		if page, _ := e.sites.ListSites(ctx, e.alex, listAll); len(page.Sites) != 0 {
			t.Errorf("rejected manifests left %d sites behind", len(page.Sites))
		}
	})
}

func TestManifestAcceptsDotfilesAndDeepPaths(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		deep := strings.Repeat("d/", 50) + "file.txt"
		files := site{".well-known/acme-challenge/token": "t", ".htaccess": "h", deep: "deep", fmt.Sprintf("%s.txt", strings.Repeat("n", 200)): "n"}
		r := e.publishNew("dotfiles", files)
		for path, want := range files {
			if got := e.readLive("dotfiles", path); got != want {
				t.Errorf("%s = %q, want %q", path, got, want)
			}
		}
		if r.Version.FileCount != len(files) {
			t.Errorf("FileCount = %d", r.Version.FileCount)
		}
	})
}
