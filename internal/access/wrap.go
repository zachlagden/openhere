package access

import "fmt"

func wrap(action string, err error) error {
	return fmt.Errorf("%s: %w", action, err)
}
