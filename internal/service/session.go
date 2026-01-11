package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"matter-core/internal/model"
	"matter-core/internal/repository"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SessionStore struct {
	mongoRepo *repository.MongoRepo
}

func NewSessionStore(mongoRepo *repository.MongoRepo) *SessionStore {
	return &SessionStore{mongoRepo: mongoRepo}
}

func (s *SessionStore) Create(ctx context.Context, userID primitive.ObjectID, role string, duration time.Duration) (string, error) {
	token, err := generateToken(32)
	if err != nil {
		return "", err
	}

	session := &model.Session{
		Token:     token,
		UserID:    userID,
		Role:      role,
		ExpiresAt: time.Now().Add(duration),
	}

	if err := s.mongoRepo.CreateSession(ctx, session); err != nil {
		return "", err
	}
	return token, nil
}

func (s *SessionStore) Get(ctx context.Context, token string) (*model.Session, error) {
	return s.mongoRepo.GetSessionByToken(ctx, token)
}

func (s *SessionStore) Delete(ctx context.Context, token string) error {
	return s.mongoRepo.DeleteSession(ctx, token)
}

func (s *SessionStore) IsValid(ctx context.Context, token string) (*model.Session, bool) {
	session, err := s.Get(ctx, token)
	if err != nil {
		return nil, false
	}
	// Explicit expiration check (MongoDB TTL may have delay)
	if time.Now().After(session.ExpiresAt) {
		return nil, false
	}
	return session, true
}

func generateToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
