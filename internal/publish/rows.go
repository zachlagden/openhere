package publish

import (
	"database/sql"
	"errors"
	"fmt"
)

func closeRows(rows *sql.Rows, cause error) error {
	if err := rows.Close(); err != nil {
		return errors.Join(cause, fmt.Errorf("close rows: %w", err))
	}
	return cause
}
