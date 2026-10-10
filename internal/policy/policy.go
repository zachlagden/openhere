package policy

import (
	"context"
	"errors"
	"fmt"

	"github.com/zachlagden/selfhost/internal/auth"
	"github.com/zachlagden/selfhost/internal/settings"
)

var ErrSharedCreationRestricted = errors.New("only the root user can create shared workspaces on this instance")

type Users interface {
	UserByID(ctx context.Context, id string) (auth.User, error)
}

func CanCreateShared(cfg settings.Settings, user auth.User) bool {
	return cfg.WhoCanCreate != settings.CreateRoot || user.IsRoot
}

func MayCreateShared(settingsSvc *settings.Service, users Users) func(ctx context.Context, userID string) error {
	return func(ctx context.Context, userID string) error {
		current, err := settingsSvc.Get(ctx)
		if err != nil {
			return fmt.Errorf("read workspace creation policy: %w", err)
		}
		if current.WhoCanCreate != settings.CreateRoot {
			return nil
		}
		user, err := users.UserByID(ctx, userID)
		if err != nil {
			return fmt.Errorf("load user for workspace creation: %w", err)
		}
		if !CanCreateShared(current, user) {
			return ErrSharedCreationRestricted
		}
		return nil
	}
}
