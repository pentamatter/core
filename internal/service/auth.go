package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"matter-core/internal/config"
	"matter-core/internal/model"
	"matter-core/internal/repository"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

type AuthService struct {
	mongoRepo    *repository.MongoRepo
	cfg          *config.Config
	githubConfig *oauth2.Config
	googleConfig *oauth2.Config
}

func NewAuthService(mongoRepo *repository.MongoRepo, cfg *config.Config) *AuthService {
	svc := &AuthService{
		mongoRepo: mongoRepo,
		cfg:       cfg,
	}

	if cfg.GitHubClientID != "" {
		svc.githubConfig = &oauth2.Config{
			ClientID:     cfg.GitHubClientID,
			ClientSecret: cfg.GitHubClientSecret,
			Endpoint:     github.Endpoint,
			RedirectURL:  cfg.OAuthRedirectURL + "/github",
			Scopes:       []string{"user:email"},
		}
	}

	if cfg.GoogleClientID != "" {
		svc.googleConfig = &oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			Endpoint:     google.Endpoint,
			RedirectURL:  cfg.OAuthRedirectURL + "/google",
			Scopes:       []string{"email", "profile"},
		}
	}

	return svc
}

// generateState creates a cryptographically secure random state for CSRF protection
// State is stored in MongoDB for distributed deployment support
func (s *AuthService) generateState(ctx context.Context) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	state := base64.URLEncoding.EncodeToString(b)

	oauthState := &model.OAuthState{
		State:     state,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	if err := s.mongoRepo.CreateOAuthState(ctx, oauthState); err != nil {
		return "", err
	}

	return state, nil
}

// ValidateState checks if the state is valid and removes it from store
func (s *AuthService) ValidateState(ctx context.Context, state string) bool {
	oauthState, err := s.mongoRepo.GetAndDeleteOAuthState(ctx, state)
	if err != nil {
		return false
	}
	return time.Now().Before(oauthState.ExpiresAt)
}

func (s *AuthService) GetAuthURL(ctx context.Context, provider string) (string, error) {
	state, err := s.generateState(ctx)
	if err != nil {
		return "", errors.New("failed to generate state")
	}

	switch provider {
	case "github":
		if s.githubConfig == nil {
			return "", errors.New("github oauth not configured")
		}
		return s.githubConfig.AuthCodeURL(state), nil
	case "google":
		if s.googleConfig == nil {
			return "", errors.New("google oauth not configured")
		}
		return s.googleConfig.AuthCodeURL(state), nil
	default:
		return "", errors.New("unsupported provider")
	}
}

func (s *AuthService) HandleCallback(ctx context.Context, provider, code string) (*model.User, error) {
	var socialBind model.SocialBind
	var err error

	switch provider {
	case "github":
		socialBind, err = s.handleGitHubCallback(ctx, code)
	case "google":
		socialBind, err = s.handleGoogleCallback(ctx, code)
	default:
		return nil, errors.New("unsupported provider")
	}

	if err != nil {
		return nil, err
	}

	// 先通过社交账号查找用户
	user, err := s.mongoRepo.GetUserBySocial(ctx, socialBind.Provider, socialBind.ProviderUserID)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	if user != nil {
		return user, nil
	}

	// 社交账号未绑定，尝试通过 email 查找已有用户
	if socialBind.Email != "" {
		user, err = s.mongoRepo.GetUserByEmail(ctx, socialBind.Email)
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}

		if user != nil {
			// 找到同 email 用户，绑定新的社交账号
			if err := s.mongoRepo.AddUserSocial(ctx, user.ID, socialBind); err != nil {
				return nil, err
			}
			user.Socials = append(user.Socials, socialBind)
			return user, nil
		}
	}

	// 创建新用户
	role := string(model.RoleUser)
	if s.cfg.AdminEmail != "" && socialBind.Email == s.cfg.AdminEmail {
		role = string(model.RoleAdmin)
	}

	user = &model.User{
		Role:     role,
		Nickname: socialBind.Name,
		Email:    socialBind.Email,
		Avatar:   socialBind.Avatar,
		Socials:  []model.SocialBind{socialBind},
	}
	if err := s.mongoRepo.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *AuthService) handleGitHubCallback(ctx context.Context, code string) (model.SocialBind, error) {
	token, err := s.githubConfig.Exchange(ctx, code)
	if err != nil {
		return model.SocialBind{}, err
	}

	client := s.githubConfig.Client(ctx, token)
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return model.SocialBind{}, err
	}
	defer resp.Body.Close()

	var ghUser struct {
		ID        int    `json:"id"`
		Login     string `json:"login"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ghUser); err != nil {
		return model.SocialBind{}, err
	}

	if ghUser.Email == "" {
		emailResp, err := client.Get("https://api.github.com/user/emails")
		if err == nil {
			defer emailResp.Body.Close()
			var emails []struct {
				Email   string `json:"email"`
				Primary bool   `json:"primary"`
			}
			if json.NewDecoder(emailResp.Body).Decode(&emails) == nil {
				for _, e := range emails {
					if e.Primary {
						ghUser.Email = e.Email
						break
					}
				}
			}
		}
	}

	return model.SocialBind{
		Provider:       "github",
		ProviderUserID: fmt.Sprintf("%d", ghUser.ID),
		Name:           ghUser.Login,
		Email:          ghUser.Email,
		Avatar:         ghUser.AvatarURL,
	}, nil
}

func (s *AuthService) handleGoogleCallback(ctx context.Context, code string) (model.SocialBind, error) {
	token, err := s.googleConfig.Exchange(ctx, code)
	if err != nil {
		return model.SocialBind{}, err
	}

	client := s.googleConfig.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return model.SocialBind{}, err
	}
	defer resp.Body.Close()

	var googleUser struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Email   string `json:"email"`
		Picture string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		return model.SocialBind{}, err
	}

	return model.SocialBind{
		Provider:       "google",
		ProviderUserID: googleUser.ID,
		Name:           googleUser.Name,
		Email:          googleUser.Email,
		Avatar:         googleUser.Picture,
	}, nil
}

func (s *AuthService) GetUserByID(ctx context.Context, userID string) (*model.User, error) {
	oid, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, err
	}
	return s.mongoRepo.GetUserByID(ctx, oid)
}

func (s *AuthService) UpdateUser(ctx context.Context, user *model.User) error {
	return s.mongoRepo.UpdateUser(ctx, user)
}
