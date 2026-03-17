package service

import (
	"errors"
	"time"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/store"
	"golang.org/x/crypto/bcrypt"
)

type authService struct {
	store  *store.Registry
}

func NewAuthService(s *store.Registry) AuthService {
	return &authService{
		store: s,
	}
}

func (s *authService) Login(username, password string) (string, error) {
	user, ok := s.store.GetUser(username)
	if !ok {
		return "", errors.New("user not found")
	}
	
	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil && user.Password != password {
		return "", errors.New("invalid credentials")
	}

	return username, nil
}

func (s *authService) VerifyAPIKey(key string) (*model.APIKey, bool) {
	k, ok := s.store.GetAPIKey(key)
	if !ok {
		return nil, false
	}
	now := time.Now().Unix()
	if k.ExpiresAt != 0 && now > k.ExpiresAt {
		return nil, false
	}
	return k, true
}

func (s *authService) GetUser(username string) (*model.User, bool) {
	return s.store.GetUser(username)
}
