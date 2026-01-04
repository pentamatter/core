package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"matter-core/internal/config"
	"matter-core/internal/model"
	"matter-core/internal/repository"

	"github.com/golang-jwt/jwt/v4"
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

func (s *AuthService) GetAuthURL(provider string) (string, error) {
	switch provider {
	case "github":
		if s.githubConfig == nil {
			return "", errors.New("github oauth not configured")
		}
		return s.githubConfig.AuthCodeURL("state"), nil
	case "google":
		if s.googleConfig == nil {
			return "", errors.New("google oauth not configured")
		}
		return s.googleConfig.AuthCodeURL("state"), nil
	default:
		return "", errors.New("unsupported provider")
	}
}

func (s *AuthService) HandleCallback(ctx context.Context, provider, code string) (*model.User, string, error) {
	var socialBind model.SocialBind
	var err error

	switch provider {
	case "github":
		socialBind, err = s.handleGitHubCallback(ctx, code)
	case "google":
		socialBind, err = s.handleGoogleCallback(ctx, code)
	default:
		return nil, "", errors.New("unsupported provider")
	}

	if err != nil {
		return nil, "", err
	}

	user, err := s.mongoRepo.GetUserBySocial(ctx, socialBind.Provider, socialBind.ProviderUserID)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			role := string(model.RoleUser)
			if s.cfg.AdminEmail != "" && socialBind.Email == s.cfg.AdminEmail {
				role = string(model.RoleAdmin)
			}

			user = &model.User{
				Role:     role,
				Nickname: socialBind.Name,
				Email:    socialBind.Email,
				Socials:  []model.SocialBind{socialBind},
			}
			if err := s.mongoRepo.CreateUser(ctx, user); err != nil {
				return nil, "", err
			}
		} else {
			return nil, "", err
		}
	}

	token, err := s.GenerateJWT(user)
	if err != nil {
		return nil, "", err
	}

	return user, token, nil
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
		ID    int    `json:"id"`
		Login string `json:"login"`
		Email string `json:"email"`
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
		ID    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		return model.SocialBind{}, err
	}

	return model.SocialBind{
		Provider:       "google",
		ProviderUserID: googleUser.ID,
		Name:           googleUser.Name,
		Email:          googleUser.Email,
	}, nil
}

type JWTClaims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func (s *AuthService) GenerateJWT(user *model.User) (string, error) {
	claims := JWTClaims{
		UserID: user.ID.Hex(),
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * 7 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWTSecret))
}

func (s *AuthService) ValidateJWT(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}

func (s *AuthService) GetUserByID(ctx context.Context, userID string) (*model.User, error) {
	oid, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, err
	}
	return s.mongoRepo.GetUserByID(ctx, oid)
}
