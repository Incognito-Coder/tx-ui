package controller

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"x-ui/internal/util/crypto"
	"x-ui/internal/web/entity"
	"x-ui/internal/web/service"
	"x-ui/internal/web/session"

	"github.com/gin-gonic/gin"
)

type updateUserForm struct {
	OldUsername string `json:"oldUsername" form:"oldUsername"`
	OldPassword string `json:"oldPassword" form:"oldPassword"`
	NewUsername string `json:"newUsername" form:"newUsername"`
	NewPassword string `json:"newPassword" form:"newPassword"`
}

type updateSecretForm struct {
	LoginSecret string `json:"loginSecret" form:"loginSecret"`
}

type SettingController struct {
	settingService service.SettingService
	userService    service.UserService
	panelService   service.PanelService
	serverService  service.ServerService
}

func NewSettingController(g *gin.RouterGroup) *SettingController {
	a := &SettingController{}
	a.initRouter(g)
	return a
}

func (a *SettingController) checkAdmin(c *gin.Context) {
	loginUser := session.GetLoginUser(c)
	if loginUser == nil || !loginUser.IsAdmin() {
		pureJsonMsg(c, http.StatusForbidden, false, I18nWeb(c, "pages.forbidden.adminRequired"))
		c.Abort()
		return
	}
	c.Next()
}

func (a *SettingController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/setting")
	g.POST("/defaultSettings", a.getDefaultSettings)

	g.Use(a.checkAdmin)

	g.POST("/all", a.getAllSetting)
	g.POST("/update", a.updateSetting)
	g.POST("/updateUser", a.updateUser)
	g.POST("/restartPanel", a.restartPanel)
	g.GET("/getDefaultJsonConfig", a.getDefaultXrayConfig)
	g.POST("/updateUserSecret", a.updateSecret)
	g.POST("/getUserSecret", a.getUserSecret)
	g.POST("/dbStatus", a.getDBStatus)
	g.GET("/dbStatus", a.getDBStatus)
	g.POST("/optimizeDatabase", a.optimizeDatabase)
	g.POST("/vacuumDatabase", a.vacuumDatabase)
	g.POST("/cleanDatabase", a.cleanDatabase)
	g.POST("/factoryReset", a.factoryReset)
	g.GET("/staff/list", a.listStaff)
	g.POST("/staff/add", a.addStaff)
	g.POST("/staff/update/:id", a.updateStaff)
	g.POST("/staff/delete/:id", a.deleteStaff)
}

func (a *SettingController) getAllSetting(c *gin.Context) {
	allSetting, err := a.settingService.GetAllSetting()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	jsonObj(c, allSetting, nil)
}

func (a *SettingController) getDefaultSettings(c *gin.Context) {
	result, err := a.settingService.GetDefaultSettings(c.Request.Host)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	jsonObj(c, result, nil)
}

func (a *SettingController) updateSetting(c *gin.Context) {
	allSetting := &entity.AllSetting{}
	err := c.ShouldBind(allSetting)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
		return
	}
	err = a.settingService.UpdateAllSetting(allSetting)
	jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
}

func (a *SettingController) updateUser(c *gin.Context) {
	form := &updateUserForm{}
	err := c.ShouldBind(form)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
		return
	}
	user := session.GetLoginUser(c)
	if user.Username != form.OldUsername || !crypto.CheckPasswordHash(user.Password, form.OldPassword) {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifyUser"), errors.New(I18nWeb(c, "pages.settings.toasts.originalUserPassIncorrect")))
		return
	}
	if form.NewUsername == "" || form.NewPassword == "" {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifyUser"), errors.New(I18nWeb(c, "pages.settings.toasts.userPassMustBeNotEmpty")))
		return
	}
	err = a.userService.UpdateUser(user.Id, form.NewUsername, form.NewPassword)
	if err == nil {
		updatedUser, errGet := a.userService.GetUserById(user.Id)
		if errGet == nil && updatedUser != nil {
			session.SetLoginUser(c, updatedUser)
		} else {
			user.Username = form.NewUsername
			user.Password, _ = crypto.HashPasswordAsBcrypt(form.NewPassword)
			session.SetLoginUser(c, user)
		}
	}
	jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifyUser"), err)
}

func (a *SettingController) restartPanel(c *gin.Context) {
	err := a.panelService.RestartPanel(time.Second * 3)
	jsonMsg(c, I18nWeb(c, "pages.settings.restartPanel"), err)
}

func (a *SettingController) updateSecret(c *gin.Context) {
	form := &updateSecretForm{}
	err := c.ShouldBind(form)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
		return
	}
	user := session.GetLoginUser(c)
	err = a.userService.UpdateUserSecret(user.Id, form.LoginSecret)
	if err == nil {
		user.LoginSecret = form.LoginSecret
		session.SetLoginUser(c, user)
	}
	jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifyUser"), err)
}

func (a *SettingController) getUserSecret(c *gin.Context) {
	loginUser := session.GetLoginUser(c)
	user := a.userService.GetUserSecret(loginUser.Id)
	if user != nil {
		jsonObj(c, user, nil)
	}
}

func (a *SettingController) getDefaultXrayConfig(c *gin.Context) {
	defaultJsonConfig, err := a.settingService.GetDefaultXrayConfig()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	jsonObj(c, defaultJsonConfig, nil)
}

func (a *SettingController) getDBStatus(c *gin.Context) {
	status, err := a.settingService.GetDBStatus()
	if err != nil {
		jsonMsg(c, "Failed to get database status", err)
		return
	}
	jsonObj(c, status, nil)
}

func (a *SettingController) optimizeDatabase(c *gin.Context) {
	err := a.settingService.OptimizeDB()
	if err != nil {
		jsonMsg(c, "Failed to optimize database", err)
		return
	}
	status, _ := a.settingService.GetDBStatus()
	jsonObj(c, status, nil)
}

func (a *SettingController) vacuumDatabase(c *gin.Context) {
	result, err := a.settingService.VacuumDB()
	if err != nil {
		jsonMsg(c, "Failed to vacuum database", err)
		return
	}
	jsonObj(c, result, nil)
}

func (a *SettingController) cleanDatabase(c *gin.Context) {
	result, err := a.settingService.CleanCorruptedRows()
	if err != nil {
		jsonMsg(c, "Failed to clean corrupted rows", err)
		return
	}
	jsonObj(c, result, nil)
}

func (a *SettingController) factoryReset(c *gin.Context) {
	err := a.settingService.FactoryReset()
	if err != nil {
		jsonMsg(c, "Failed to perform factory reset", err)
		return
	}
	_ = a.serverService.RestartXrayService()

	// Destroy the current session immediately upon factory reset
	session.ClearSession(c)

	jsonMsg(c, "Factory reset completed successfully", nil)
}

type StaffForm struct {
	Username        string `json:"username" form:"username"`
	Password        string `json:"password" form:"password"`
	Role            string `json:"role" form:"role"`
	Permissions     string `json:"permissions" form:"permissions"`
	AllowedInbounds string `json:"allowedInbounds" form:"allowedInbounds"`
	CanResetTraffic bool   `json:"canResetTraffic" form:"canResetTraffic"`
	CanBackupDB     bool   `json:"canBackupDB" form:"canBackupDB"`
	CanRestoreDB    bool   `json:"canRestoreDB" form:"canRestoreDB"`
	CanOperateXray  bool   `json:"canOperateXray" form:"canOperateXray"`
	CanOperateGeo   bool   `json:"canOperateGeo" form:"canOperateGeo"`
	CanViewLinks    bool   `json:"canViewLinks" form:"canViewLinks"`
	Status          string `json:"status" form:"status"`
}

func (a *SettingController) listStaff(c *gin.Context) {
	loginUser := session.GetLoginUser(c)
	if loginUser == nil || !loginUser.IsAdmin() {
		pureJsonMsg(c, http.StatusForbidden, false, I18nWeb(c, "pages.forbidden.adminRequired"))
		return
	}
	users, err := a.userService.GetAllUsers()
	if err != nil {
		jsonMsg(c, err.Error(), err)
		return
	}
	for _, u := range users {
		u.Password = ""
	}
	jsonObj(c, users, nil)
}

func (a *SettingController) addStaff(c *gin.Context) {
	loginUser := session.GetLoginUser(c)
	if loginUser == nil || !loginUser.IsAdmin() {
		pureJsonMsg(c, http.StatusForbidden, false, I18nWeb(c, "pages.forbidden.adminRequired"))
		return
	}
	f := &StaffForm{}
	if err := c.ShouldBind(f); err != nil {
		jsonMsg(c, "Invalid request", err)
		return
	}
	user, err := a.userService.CreateStaffUser(
		f.Username, f.Password, f.Role, f.Permissions, f.AllowedInbounds,
		f.CanResetTraffic, f.CanBackupDB, f.CanRestoreDB, f.CanOperateXray, f.CanOperateGeo, f.CanViewLinks,
	)
	if err != nil {
		jsonMsg(c, err.Error(), err)
		return
	}
	user.Password = ""
	jsonObj(c, user, nil)
}

func (a *SettingController) updateStaff(c *gin.Context) {
	loginUser := session.GetLoginUser(c)
	if loginUser == nil || !loginUser.IsAdmin() {
		pureJsonMsg(c, http.StatusForbidden, false, I18nWeb(c, "pages.forbidden.adminRequired"))
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid user id", err)
		return
	}
	f := &StaffForm{}
	if err := c.ShouldBind(f); err != nil {
		jsonMsg(c, "Invalid request", err)
		return
	}
	err = a.userService.UpdateStaffUser(
		id, f.Username, f.Password, f.Role, f.Permissions, f.AllowedInbounds,
		f.CanResetTraffic, f.CanBackupDB, f.CanRestoreDB, f.CanOperateXray, f.CanOperateGeo, f.CanViewLinks, f.Status,
	)
	if err != nil {
		jsonMsg(c, err.Error(), err)
		return
	}
	jsonMsg(c, "Staff member updated successfully", nil)
}

func (a *SettingController) deleteStaff(c *gin.Context) {
	loginUser := session.GetLoginUser(c)
	if loginUser == nil || !loginUser.IsAdmin() {
		pureJsonMsg(c, http.StatusForbidden, false, I18nWeb(c, "pages.forbidden.adminRequired"))
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid user id", err)
		return
	}
	if id == loginUser.Id {
		jsonMsg(c, "Cannot delete your own logged-in account", errors.New("self deletion"))
		return
	}
	err = a.userService.DeleteStaffUser(id)
	if err != nil {
		jsonMsg(c, err.Error(), err)
		return
	}
	jsonMsg(c, "Staff member deleted successfully", nil)
}

