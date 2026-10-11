package analytics

import (
	"strconv"

	"github.com/zachlagden/openhere/internal/workspaces"
)

type builder struct {
	args []any
}

func (b *builder) arg(value any) string {
	b.args = append(b.args, value)
	return "$" + strconv.Itoa(len(b.args))
}

type scope func(b *builder, column string) string

func siteScope(siteID string) scope {
	return func(b *builder, column string) string {
		return column + " = " + b.arg(siteID)
	}
}

func workspaceScope(userID, workspaceID string) scope {
	return func(b *builder, column string) string {
		user := b.arg(userID)
		return column + " IN (SELECT s.id FROM sites s WHERE s.workspace_id = " + b.arg(workspaceID) +
			" AND " + workspaces.SiteCoverSQL("s", user) + ")"
	}
}

func instanceScope() scope {
	return func(*builder, string) string {
		return "1 = 1"
	}
}
