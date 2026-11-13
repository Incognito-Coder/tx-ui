package controller

import (
	"x-ui/internal/web/session"

	"github.com/gin-gonic/gin"
)

type XUIController struct {
	BaseController

	settingController     *SettingController
	xraySettingController *XraySettingController
}

func NewXUIController(g *gin.RouterGroup) *XUIController {
	a := &XUIController{}
	a.initRouter(g)
	return a
}

func (a *XUIController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/panel")
	g.Use(a.checkLogin)

	g.GET("/", a.index)
	g.GET("/inbounds", a.inbounds)
	g.GET("/clients", a.clients)
	g.GET("/settings", a.settings)
	g.GET("/xray", a.xraySettings)
	g.GET("/docs", a.docs)
	g.GET("/api-docs", a.docs)

	a.settingController = NewSettingController(g)
	a.xraySettingController = NewXraySettingController(g)
}

func (a *XUIController) index(c *gin.Context) {
	html(c, "index.html", "pages.index.title", nil)
}

func (a *XUIController) inbounds(c *gin.Context) {
	html(c, "inbounds.html", "pages.inbounds.title", nil)
}

func (a *XUIController) clients(c *gin.Context) {
	html(c, "clients.html", "pages.clients.title", nil)
}

func (a *XUIController) settings(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		render403(c, "pages.forbidden.settingsDenied")
		return
	}
	html(c, "settings.html", "pages.settings.title", nil)
}

func (a *XUIController) xraySettings(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		render403(c, "pages.forbidden.xrayDenied")
		return
	}
	html(c, "xray.html", "pages.xray.title", nil)
}

func (a *XUIController) docs(c *gin.Context) {
	html(c, "docs.html", "pages.docs.title", nil)
}
