package settings

import (
	"database/sql"
	"errors"
	"fmt"
)

func closeAfter(rows *sql.Rows, cause error) error {
	if err := rows.Close(); err != nil {
		return errors.Join(cause, fmt.Errorf("close setting rows: %w", err))
	}
	return cause
}
