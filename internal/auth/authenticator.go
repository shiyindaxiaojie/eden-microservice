package auth

import (
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Authenticator interface {
	Login(username, password string) (string, error)
	VerifyAPIKey(key string) (*APIKey, bool)
	GetUser(username string) (*User, bool)
	UpdateProfile(username, nickname, phone, email string) error
	UpdatePassword(username, oldPassword, newPassword string) error
	UpdateGuideStatus(username string, completed bool) error
}

type authenticator struct {
	directory *Directory
}

func NewAuthenticator(directory *Directory) Authenticator {
	return &authenticator{
		directory: directory,
	}
}

func (a *authenticator) Login(username, password string) (string, error) {
	user, ok := a.directory.GetUser(username)
	if !ok {
		return "", errors.New("user not found")
	}

	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil && user.Password != password {
		return "", errors.New("invalid credentials")
	}

	return username, nil
}

func (a *authenticator) VerifyAPIKey(key string) (*APIKey, bool) {
	k, ok := a.directory.GetAPIKey(key)
	if !ok {
		return nil, false
	}
	now := time.Now().Unix()
	if k.ExpiresAt != 0 && now > k.ExpiresAt {
		return nil, false
	}
	return k, true
}

func (a *authenticator) GetUser(username string) (*User, bool) {
	return a.directory.GetUser(username)
}

func (a *authenticator) UpdateProfile(username, nickname, phone, email string) error {
	user, ok := a.directory.GetUser(username)
	if !ok {
		return errors.New("user not found")
	}
	user.Nickname = nickname
	user.Phone = phone
	user.Email = email
	a.directory.AddUser(user)
	a.directory.Save()
	return nil
}

func (a *authenticator) UpdatePassword(username, oldPassword, newPassword string) error {
	user, ok := a.directory.GetUser(username)
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
	a.directory.AddUser(user)
	a.directory.Save()
	return nil
}

func (a *authenticator) UpdateGuideStatus(username string, completed bool) error {
	user, ok := a.directory.GetUser(username)
	if !ok {
		return errors.New("user not found")
	}
	user.GuideCompleted = completed
	a.directory.AddUser(user)
	a.directory.Save()
	return nil
}
