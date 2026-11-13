package session

import (
	"encoding/gob"
	"net/http"

	"x-ui/internal/database"
	"x-ui/internal/database/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const (
	loginUserKey   = "LOGIN_USER"
	defaultPath    = "/"
	ginUserContext = "CURRENT_LOGIN_USER"
)

func init() {
	gob.Register(model.User{})
}

func SetLoginUser(c *gin.Context, user *model.User) {
	if user == nil {
		return
	}
	s := sessions.Default(c)
	s.Set(loginUserKey, *user)
	c.Set(ginUserContext, user)
}

func SetMaxAge(c *gin.Context, maxAge int) {
	s := sessions.Default(c)
	s.Options(sessions.Options{
		Path:     defaultPath,
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func GetLoginUser(c *gin.Context) *model.User {
	if cached, exists := c.Get(ginUserContext); exists && cached != nil {
		if u, ok := cached.(*model.User); ok {
			return u
		}
	}

	s := sessions.Default(c)
	obj := s.Get(loginUserKey)
	if obj == nil {
		return nil
	}
	user, ok := obj.(model.User)
	if !ok {
		ClearSession(c)
		return nil
	}

	// Verify that the user still exists in DB and credentials haven't been wiped/reset
	db := database.GetDB()
	if db != nil {
		var dbUser model.User
		err := db.Select("id, username, password, role, permissions, allowed_inbounds, can_reset_traffic, can_backup_db, can_restore_db, can_operate_xray, can_operate_geo, can_view_links, status").
			Where("id = ? AND username = ?", user.Id, user.Username).
			First(&dbUser).Error
		if err != nil || dbUser.Password != user.Password || dbUser.Status == "disabled" {
			ClearSession(c)
			return nil
		}
		user = dbUser
	}

	c.Set(ginUserContext, &user)
	return &user
}

func IsLogin(c *gin.Context) bool {
	return GetLoginUser(c) != nil
}

func ClearSession(c *gin.Context) {
	c.Set(ginUserContext, nil)
	s := sessions.Default(c)
	s.Clear()
	s.Options(sessions.Options{
		Path:     defaultPath,
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	_ = s.Save()

	// Clear session cookie across default root and current base_path
	c.SetCookie("tx-ui", "", -1, defaultPath, "", false, true)
	basePath := c.GetString("base_path")
	if basePath != "" && basePath != defaultPath {
		c.SetCookie("tx-ui", "", -1, basePath, "", false, true)
	}
}
