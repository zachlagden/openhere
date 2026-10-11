package sites

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/zachlagden/openhere/internal/ids"
	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/validate"
	"github.com/zachlagden/openhere/internal/workspaces"
)

const folderColumns = "id, workspace_id, name, created_at, updated_at"

func scanFolder(row interface{ Scan(dest ...any) error }) (Folder, error) {
	var f Folder
	err := row.Scan(&f.ID, &f.WorkspaceID, &f.Name, &f.CreatedAt, &f.UpdatedAt)
	return f, err
}

func (s *Service) ListFolders(ctx context.Context, userID, workspaceRef string) ([]Folder, error) {
	acc, err := s.ws.Access(ctx, userID, workspaceRef)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT "+folderColumns+" FROM folders WHERE workspace_id = $1 ORDER BY name_key, id", acc.Workspace.ID)
	if err != nil {
		return nil, fmt.Errorf("list folders: %w", err)
	}
	folders := []Folder{}
	for rows.Next() {
		f, err := scanFolder(rows)
		if err != nil {
			return nil, errors.Join(fmt.Errorf("scan folder: %w", err), rows.Close())
		}
		if f.Role = acc.FolderRole(f.ID); f.Role != workspaces.RoleNone {
			folders = append(folders, f)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Join(fmt.Errorf("read folders: %w", err), rows.Close())
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close folder rows: %w", err)
	}
	return folders, nil
}

func (s *Service) CreateFolder(ctx context.Context, userID, workspaceRef, name string) (Folder, error) {
	acc, err := s.ws.Authorize(ctx, userID, workspaceRef, workspaces.RoleWrite)
	if err != nil {
		return Folder{}, err
	}
	clean, err := validate.Line("Name", name, MaxFolderNameLength)
	if err != nil {
		return Folder{}, err
	}
	id, err := ids.New()
	if err != nil {
		return Folder{}, err
	}
	now := s.clock()
	_, err = s.db.ExecContext(ctx,
		"INSERT INTO folders (id, workspace_id, name, name_key, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $5)",
		id, acc.Workspace.ID, clean, nameKey(clean), now)
	if store.IsUniqueViolation(err) {
		return Folder{}, ErrFolderNameTaken
	}
	if err != nil {
		return Folder{}, fmt.Errorf("insert folder: %w", err)
	}
	return Folder{ID: id, WorkspaceID: acc.Workspace.ID, Name: clean, Role: acc.WorkspaceRole(), CreatedAt: now, UpdatedAt: now}, nil
}

func (s *Service) RenameFolder(ctx context.Context, userID, workspaceRef, folderID, name string) (Folder, error) {
	folder, err := s.managedFolder(ctx, userID, workspaceRef, folderID, workspaces.RoleWrite)
	if err != nil {
		return Folder{}, err
	}
	clean, err := validate.Line("Name", name, MaxFolderNameLength)
	if err != nil {
		return Folder{}, err
	}
	now := s.clock()
	_, err = s.db.ExecContext(ctx, "UPDATE folders SET name = $1, name_key = $2, updated_at = $3 WHERE id = $4", clean, nameKey(clean), now, folder.ID)
	if store.IsUniqueViolation(err) {
		return Folder{}, ErrFolderNameTaken
	}
	if err != nil {
		return Folder{}, fmt.Errorf("rename folder: %w", err)
	}
	folder.Name = clean
	folder.UpdatedAt = now
	return folder, nil
}

func (s *Service) DeleteFolder(ctx context.Context, userID, workspaceRef, folderID string, force bool) error {
	folder, err := s.managedFolder(ctx, userID, workspaceRef, folderID, workspaces.RoleAdmin)
	if err != nil {
		return err
	}
	return store.WithTx(ctx, s.db, func(q store.Querier) error {
		if !force {
			if err := refuseWhenGranted(ctx, q, folder.ID); err != nil {
				return err
			}
		}
		if _, err := q.ExecContext(ctx, "UPDATE sites SET folder_id = NULL, updated_at = $1 WHERE folder_id = $2", s.clock(), folder.ID); err != nil {
			return fmt.Errorf("unfile sites: %w", err)
		}
		if _, err := q.ExecContext(ctx, "DELETE FROM folders WHERE id = $1", folder.ID); err != nil {
			return fmt.Errorf("delete folder: %w", err)
		}
		return nil
	})
}

func refuseWhenGranted(ctx context.Context, q store.Querier, folderID string) error {
	var grants int
	err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM workspace_access WHERE folder_id = $1", folderID).Scan(&grants)
	if err != nil {
		return fmt.Errorf("count folder grants: %w", err)
	}
	if grants > 0 {
		return ErrFolderHasGrants
	}
	return nil
}

func (s *Service) managedFolder(ctx context.Context, userID, workspaceRef, folderID string, need workspaces.Role) (Folder, error) {
	acc, err := s.ws.Access(ctx, userID, workspaceRef)
	if err != nil {
		return Folder{}, err
	}
	folder, err := loadFolder(ctx, s.db, acc.Workspace.ID, folderID)
	if err != nil {
		return Folder{}, err
	}
	folder.Role = acc.FolderRole(folder.ID)
	if folder.Role == workspaces.RoleNone {
		return Folder{}, workspaces.ErrNotFound
	}
	if err := workspaces.Require(acc.WorkspaceRole(), need); err != nil {
		return Folder{}, err
	}
	return folder, nil
}

func loadFolder(ctx context.Context, q store.Querier, workspaceID, folderID string) (Folder, error) {
	f, err := scanFolder(q.QueryRowContext(ctx, "SELECT "+folderColumns+" FROM folders WHERE workspace_id = $1 AND id = $2", workspaceID, folderID))
	if errors.Is(err, sql.ErrNoRows) {
		return Folder{}, workspaces.ErrNotFound
	}
	if err != nil {
		return Folder{}, fmt.Errorf("load folder: %w", err)
	}
	return f, nil
}

func nameKey(name string) string {
	return strings.ToLower(name)
}

func (s *Service) FindFolder(ctx context.Context, workspaceID, ref string) (Folder, error) {
	f, err := scanFolder(s.db.QueryRowContext(ctx,
		"SELECT "+folderColumns+" FROM folders WHERE workspace_id = $1 AND (id = $2 OR name_key = $3) ORDER BY CASE WHEN id = $2 THEN 0 ELSE 1 END LIMIT 1",
		workspaceID, ref, nameKey(ref)))
	if errors.Is(err, sql.ErrNoRows) {
		return Folder{}, workspaces.ErrNotFound
	}
	if err != nil {
		return Folder{}, fmt.Errorf("find folder: %w", err)
	}
	return f, nil
}
