package service

import (
	"errors"

	"x-ui/internal/database"
	"x-ui/internal/database/model"
	"x-ui/internal/logger"
	"x-ui/internal/util/crypto"

	"gorm.io/gorm"
)

type UserService struct{}

func (s *UserService) GetFirstUser() (*model.User, error) {
	db := database.GetDB()

	user := &model.User{}
	err := db.Model(model.User{}).
		First(user).
		Error
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) CheckUser(username string, password string, secret string) *model.User {
	db := database.GetDB()

	user := &model.User{}

	err := db.Model(model.User{}).
		Where("username = ? and login_secret = ?", username, secret).
		First(user).
		Error
	if err == gorm.ErrRecordNotFound {
		return nil
	} else if err != nil {
		logger.Warning("check user err:", err)
		return nil
	}

	if user.Status == "disabled" {
		logger.Warningf("login attempted by disabled user: %s", username)
		return nil
	}

	if crypto.CheckPasswordHash(user.Password, password) {
		return user
	}

	return nil
}

func (s *UserService) UpdateUser(id int, username string, password string) error {
	db := database.GetDB()
	hashedPassword, err := crypto.HashPasswordAsBcrypt(password)

	if err != nil {
		return err
	}

	return db.Model(model.User{}).
		Where("id = ?", id).
		Updates(map[string]any{"username": username, "password": hashedPassword}).
		Error
}

func (s *UserService) UpdateUserSecret(id int, secret string) error {
	db := database.GetDB()
	return db.Model(model.User{}).
		Where("id = ?", id).
		Update("login_secret", secret).
		Error
}

func (s *UserService) RemoveUserSecret() error {
	db := database.GetDB()
	return db.Model(model.User{}).
		Where("1 = 1").
		Update("login_secret", "").
		Error
}

func (s *UserService) GetUserSecret(id int) *model.User {
	db := database.GetDB()
	user := &model.User{}
	err := db.Model(model.User{}).
		Where("id = ?", id).
		First(user).
		Error
	if err == gorm.ErrRecordNotFound {
		return nil
	}
	return user
}

func (s *UserService) CheckSecretExistence() (bool, error) {
	db := database.GetDB()

	var count int64
	err := db.Model(model.User{}).
		Where("login_secret IS NOT NULL").
		Count(&count).
		Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (s *UserService) UpdateFirstUser(username string, password string) error {
	if username == "" {
		return errors.New("username can not be empty")
	} else if password == "" {
		return errors.New("password can not be empty")
	}
	hashedPassword, er := crypto.HashPasswordAsBcrypt(password)

	if er != nil {
		return er
	}

	db := database.GetDB()
	user := &model.User{}
	err := db.Model(model.User{}).First(user).Error
	if database.IsNotFound(err) {
		user.Username = username
		user.Password = hashedPassword
		return db.Model(model.User{}).Create(user).Error
	} else if err != nil {
		return err
	}
	user.Username = username
	user.Password = hashedPassword
	return db.Save(user).Error
}

func (s *UserService) GetAllUsers() ([]*model.User, error) {
	db := database.GetDB()
	var users []*model.User
	err := db.Model(&model.User{}).Order("id asc").Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (s *UserService) GetUserById(id int) (*model.User, error) {
	db := database.GetDB()
	user := &model.User{}
	err := db.Model(&model.User{}).Where("id = ?", id).First(user).Error
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) CreateStaffUser(username, password, role, permissions, allowedInbounds string, canResetTraffic, canBackupDB, canRestoreDB, canOperateXray, canOperateGeo, canViewLinks bool) (*model.User, error) {
	if username == "" {
		return nil, errors.New("username cannot be empty")
	}
	if password == "" {
		return nil, errors.New("password cannot be empty")
	}

	db := database.GetDB()
	var count int64
	db.Model(&model.User{}).Where("username = ?", username).Count(&count)
	if count > 0 {
		return nil, errors.New("username already exists")
	}

	hashedPassword, err := crypto.HashPasswordAsBcrypt(password)
	if err != nil {
		return nil, err
	}

	if role == "" {
		role = "staff"
	}
	if permissions == "" {
		permissions = `["inbounds:read","clients:read","clients:view_links"]`
	}
	if allowedInbounds == "" {
		allowedInbounds = "*"
	}

	if role == "viewer" {
		canResetTraffic = false
		canBackupDB = false
		canRestoreDB = false
		canOperateXray = false
		canOperateGeo = false
		canViewLinks = true
	} else if role == "admin" {
		canResetTraffic = true
		canBackupDB = true
		canRestoreDB = true
		canOperateXray = true
		canOperateGeo = true
		canViewLinks = true
		permissions = "*"
		allowedInbounds = "*"
	}

	user := &model.User{
		Username:        username,
		Password:        hashedPassword,
		Role:            role,
		Permissions:     permissions,
		AllowedInbounds: allowedInbounds,
		CanResetTraffic: canResetTraffic,
		CanBackupDB:     canBackupDB,
		CanRestoreDB:    canRestoreDB,
		CanOperateXray:  canOperateXray,
		CanOperateGeo:   canOperateGeo,
		CanViewLinks:    canViewLinks,
		Status:          "active",
	}

	err = db.Create(user).Error
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) UpdateStaffUser(id int, username, password, role, permissions, allowedInbounds string, canResetTraffic, canBackupDB, canRestoreDB, canOperateXray, canOperateGeo, canViewLinks bool, status string) error {
	db := database.GetDB()
	user, err := s.GetUserById(id)
	if err != nil {
		return err
	}

	if username != "" && username != user.Username {
		var count int64
		db.Model(&model.User{}).Where("username = ? and id != ?", username, id).Count(&count)
		if count > 0 {
			return errors.New("username already taken by another account")
		}
		user.Username = username
	}

	if password != "" {
		hashed, err := crypto.HashPasswordAsBcrypt(password)
		if err != nil {
			return err
		}
		user.Password = hashed
	}

	if role != "" {
		user.Role = role
	}
	if permissions != "" {
		user.Permissions = permissions
	}
	if allowedInbounds != "" {
		user.AllowedInbounds = allowedInbounds
	}

	if user.Role == "viewer" {
		user.CanResetTraffic = false
		user.CanBackupDB = false
		user.CanRestoreDB = false
		user.CanOperateXray = false
		user.CanOperateGeo = false
		user.CanViewLinks = true
	} else if user.Role == "admin" {
		user.CanResetTraffic = true
		user.CanBackupDB = true
		user.CanRestoreDB = true
		user.CanOperateXray = true
		user.CanOperateGeo = true
		user.CanViewLinks = true
		user.Permissions = "*"
		user.AllowedInbounds = "*"
	} else {
		user.CanResetTraffic = canResetTraffic
		user.CanBackupDB = canBackupDB
		user.CanRestoreDB = canRestoreDB
		user.CanOperateXray = canOperateXray
		user.CanOperateGeo = canOperateGeo
		user.CanViewLinks = canViewLinks
	}

	if status != "" {
		user.Status = status
	}

	return db.Save(user).Error
}

func (s *UserService) DeleteStaffUser(id int) error {
	db := database.GetDB()
	var totalAdmins int64
	db.Model(&model.User{}).Where("role = ? OR role = ''", "admin").Count(&totalAdmins)

	user, err := s.GetUserById(id)
	if err != nil {
		return err
	}

	if user.IsAdmin() && totalAdmins <= 1 {
		return errors.New("cannot delete the only remaining admin account")
	}

	return db.Delete(&model.User{}, id).Error
}
