package repository

import (
	"context"

	"github.com/safrizal997/travapp-auth-service/internal/domain/entity"
)

type LoginHistoryRepository interface {
	Create(ctx context.Context, history *entity.AuthLoginHistory) error
}
