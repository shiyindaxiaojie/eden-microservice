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

func (s *authService) UpdateProfile(username, nickname, phone, email string) error {
	user, ok := s.store.GetUser(username)
	if !ok {
		return errors.New("user not found")
	}
	user.Nickname = nickname
	user.Phone = phone
	user.Email = email
	s.store.AddUser(user) // store.AddUser will call save internally
	return nil
}

func (s *authService) UpdatePassword(username, oldPassword, newPassword string) error {
	user, ok := s.store.GetUser(username)
	if !ok {
		return errors.New("user not found")
	}

	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPassword))
	if err != nil && user.Password != oldPassword {
		return errors.New("current password incorrect")
	}

	// In this project it seems sometimes plain text is used if bcrypt fails or is not applied yet
	// But let's use bcrypt for new password
	hashed, _ := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	user.Password = string(hashed)
	s.store.AddUser(user)
	return nil
}
