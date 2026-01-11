package service

import (
	"testing"

	"matter-core/internal/model"
	"matter-core/internal/testutil"
)

func TestAuthService(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup(t)

	ctx := env.Context()
	svc := NewAuthService(env.MongoRepo, env.Config)

	t.Run("GetAuthURL_UnconfiguredProvider", func(t *testing.T) {
		// GitHub and Google are not configured in test config
		_, err := svc.GetAuthURL(ctx, "github")
		if err == nil {
			t.Error("Expected error for unconfigured provider")
		}
	})

	t.Run("GetAuthURL_UnsupportedProvider", func(t *testing.T) {
		_, err := svc.GetAuthURL(ctx, "facebook")
		if err == nil {
			t.Error("Expected error for unsupported provider")
		}
	})

	t.Run("ValidateState_Invalid", func(t *testing.T) {
		valid := svc.ValidateState(ctx, "invalid-state")
		if valid {
			t.Error("Expected invalid state to return false")
		}
	})

	t.Run("HandleCallback_UnsupportedProvider", func(t *testing.T) {
		_, err := svc.HandleCallback(ctx, "facebook", "code")
		if err == nil {
			t.Error("Expected error for unsupported provider")
		}
	})
}

func TestAuthService_GetUserByID(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup(t)

	ctx := env.Context()
	svc := NewAuthService(env.MongoRepo, env.Config)

	// Create a user
	user := &model.User{
		Role:     "user",
		Nickname: "testuser",
		Email:    "test@example.com",
	}
	_ = env.MongoRepo.CreateUser(ctx, user)

	t.Run("GetUserByID_Valid", func(t *testing.T) {
		got, err := svc.GetUserByID(ctx, user.ID.Hex())
		if err != nil {
			t.Fatalf("GetUserByID failed: %v", err)
		}

		if got.Nickname != user.Nickname {
			t.Errorf("Nickname mismatch: got %s, want %s", got.Nickname, user.Nickname)
		}
	})

	t.Run("GetUserByID_InvalidID", func(t *testing.T) {
		_, err := svc.GetUserByID(ctx, "invalid-id")
		if err == nil {
			t.Error("Expected error for invalid ID")
		}
	})

	t.Run("GetUserByID_NotFound", func(t *testing.T) {
		_, err := svc.GetUserByID(ctx, "000000000000000000000000")
		if err == nil {
			t.Error("Expected error for non-existent user")
		}
	})
}

func TestAuthService_UpdateUser(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup(t)

	ctx := env.Context()
	svc := NewAuthService(env.MongoRepo, env.Config)

	// Create a user
	user := &model.User{
		Role:     "user",
		Nickname: "original",
		Email:    "update@example.com",
	}
	_ = env.MongoRepo.CreateUser(ctx, user)

	t.Run("UpdateUser", func(t *testing.T) {
		user.Nickname = "updated"
		err := svc.UpdateUser(ctx, user)
		if err != nil {
			t.Fatalf("UpdateUser failed: %v", err)
		}

		got, _ := svc.GetUserByID(ctx, user.ID.Hex())
		if got.Nickname != "updated" {
			t.Errorf("Expected nickname 'updated', got %s", got.Nickname)
		}
	})
}
