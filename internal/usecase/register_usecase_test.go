package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/safrizal997/travapp-auth-service/internal/domain/entity"
	"github.com/safrizal997/travapp-auth-service/internal/mocks"
	"github.com/safrizal997/travapp-auth-service/internal/testutil"
	"go.uber.org/mock/gomock"
)

func newRegisterUCWithMocks(t *testing.T, ctrl *gomock.Controller) (
	*RegisterUseCase,
	*mocks.MockCredentialRepository,
	*mocks.MockTenantRepository,
	*mocks.MockRoleRepository,
	*mocks.MockEmailVerificationStore,
) {
	credRepo := mocks.NewMockCredentialRepository(ctrl)
	tenantRepo := mocks.NewMockTenantRepository(ctrl)
	roleRepo := mocks.NewMockRoleRepository(ctrl)
	evStore := mocks.NewMockEmailVerificationStore(ctrl)

	uc := NewRegisterUseCase(
		credRepo, tenantRepo, roleRepo,
		testutil.NewTestHasher(),
		testutil.NewTestLogger(),
		evStore,
		"http://localhost:8080",
	)

	return uc, credRepo, tenantRepo, roleRepo, evStore
}

func TestRegisterUseCase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, tenantRepo, roleRepo, evStore := newRegisterUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	tenant := &entity.AuthTenant{ID: tenantID, IsActive: true}
	defaultRole := &entity.AuthRole{ID: uuid.New(), Code: "traveler", IsDefault: true}

	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(tenant, nil)
	credRepo.EXPECT().GetByEmailAndTenant(gomock.Any(), "new@example.com", tenantID, "email").Return(nil, nil)
	credRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	roleRepo.EXPECT().GetDefaultRole(gomock.Any(), tenantID).Return(defaultRole, nil)
	roleRepo.EXPECT().AssignRole(gomock.Any(), gomock.Any()).Return(nil)
	evStore.EXPECT().CreateEmailVerification(gomock.Any(), gomock.Any()).Return(nil)

	input := RegisterInput{
		Email:                "new@example.com",
		Password:             "password123",
		PasswordConfirmation: "password123",
		TenantID:             tenantID,
	}

	output, err := uc.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.CredentialID == uuid.Nil {
		t.Error("credential ID should not be nil")
	}
	if output.VerificationToken == "" {
		t.Error("verification token should not be empty")
	}
}

func TestRegisterUseCase_EmailConflict(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, tenantRepo, _, _ := newRegisterUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	tenant := &entity.AuthTenant{ID: tenantID, IsActive: true}
	existing := &entity.AuthCredential{ID: uuid.New()}

	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(tenant, nil)
	credRepo.EXPECT().GetByEmailAndTenant(gomock.Any(), "existing@example.com", tenantID, "email").Return(existing, nil)

	input := RegisterInput{
		Email:    "existing@example.com",
		Password: "password123",
		TenantID: tenantID,
	}

	_, err := uc.Execute(context.Background(), input)
	assertAppErrorCode(t, err, 409)
}

func TestRegisterUseCase_InvalidTenant(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, _, tenantRepo, _, _ := newRegisterUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(nil, nil)

	input := RegisterInput{
		Email:    "new@example.com",
		Password: "password123",
		TenantID: tenantID,
	}

	_, err := uc.Execute(context.Background(), input)
	assertAppErrorCode(t, err, 400)
}

func TestRegisterUseCase_NoDefaultRole(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, tenantRepo, roleRepo, evStore := newRegisterUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	tenant := &entity.AuthTenant{ID: tenantID, IsActive: true}

	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(tenant, nil)
	credRepo.EXPECT().GetByEmailAndTenant(gomock.Any(), "new@example.com", tenantID, "email").Return(nil, nil)
	credRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	roleRepo.EXPECT().GetDefaultRole(gomock.Any(), tenantID).Return(nil, nil)
	evStore.EXPECT().CreateEmailVerification(gomock.Any(), gomock.Any()).Return(nil)

	input := RegisterInput{
		Email:    "new@example.com",
		Password: "password123",
		TenantID: tenantID,
	}

	output, err := uc.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.CredentialID == uuid.Nil {
		t.Error("credential ID should not be nil")
	}
}

func TestRegisterUseCase_VerificationStoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, tenantRepo, roleRepo, evStore := newRegisterUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	tenant := &entity.AuthTenant{ID: tenantID, IsActive: true}

	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(tenant, nil)
	credRepo.EXPECT().GetByEmailAndTenant(gomock.Any(), "new@example.com", tenantID, "email").Return(nil, nil)
	credRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	roleRepo.EXPECT().GetDefaultRole(gomock.Any(), tenantID).Return(nil, nil)
	evStore.EXPECT().CreateEmailVerification(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

	input := RegisterInput{
		Email:    "new@example.com",
		Password: "password123",
		TenantID: tenantID,
	}

	_, err := uc.Execute(context.Background(), input)
	assertAppErrorCode(t, err, 500)
}

func TestRegisterUseCase_CreateCredentialError(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, tenantRepo, _, _ := newRegisterUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	tenant := &entity.AuthTenant{ID: tenantID, IsActive: true}

	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(tenant, nil)
	credRepo.EXPECT().GetByEmailAndTenant(gomock.Any(), "new@example.com", tenantID, "email").Return(nil, nil)
	credRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

	input := RegisterInput{
		Email:    "new@example.com",
		Password: "password123",
		TenantID: tenantID,
	}

	_, err := uc.Execute(context.Background(), input)
	assertAppErrorCode(t, err, 500)
}

func TestRegisterUseCase_TenantRepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, _, tenantRepo, _, _ := newRegisterUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(nil, errors.New("db error"))

	input := RegisterInput{
		Email:    "new@example.com",
		Password: "password123",
		TenantID: tenantID,
	}

	_, err := uc.Execute(context.Background(), input)
	assertAppErrorCode(t, err, 500)
}
