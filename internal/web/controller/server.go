package controller

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"x-ui/internal/web/global"
	"x-ui/internal/web/service"

	"github.com/gin-gonic/gin"
)

var filenameRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-.]+$`)

type ServerController struct {
	BaseController

	serverService  service.ServerService
	settingService service.SettingService

	lastStatus        *service.Status
	lastGetStatusTime time.Time

	lastVersions        []string
	lastGetVersionsTime time.Time
}

func NewServerController(g *gin.RouterGroup) *ServerController {
	a := &ServerController{
		lastGetStatusTime: time.Now(),
	}
	a.initRouter(g)
	a.startTask()
	return a
}

func (a *ServerController) initRouter(g *gin.RouterGroup) {
	g.GET("/status", a.status)
	g.GET("/getXrayMetrics", a.getXrayMetrics)
	g.GET("/getXrayVersion", a.getXrayVersion)
	g.GET("/getConfigJson", a.getConfigJson)
	g.GET("/getDb", a.getDb)
	g.GET("/getNewUUID", a.getNewUUID)
	g.GET("/getNewX25519Cert", a.getNewX25519Cert)
	g.GET("/getNewCert", a.getNewCert)
	g.GET("/getNewmldsa65", a.getNewmldsa65)
	g.GET("/getNewmlkem768", a.getNewmlkem768)
	g.GET("/getNewVlessEnc", a.getNewVlessEnc)

	g.POST("/stopXrayService", a.stopXrayService)
	g.POST("/restartXrayService", a.restartXrayService)
	g.POST("/installPanel", a.installPanel)
	g.POST("/installXray/:version", a.installXray)
	g.POST("/updateGeoFiles", a.updateGeoFiles)
	g.POST("/logs/:count", a.getLogs)
	g.POST("/xraylogs/:count", a.getXrayLogs)
	g.POST("/importDB", a.importDB)
	g.POST("/getNewEchCert", a.getNewEchCert)
	g.POST("/setTunnel/:ip/:port/:user/:password", a.setTunnel)
	g.GET("/getSubTemplates", a.getSubTemplates)
	g.POST("/applySubTemplate", a.applySubTemplate)
	g.POST("/resetSubTemplate", a.resetSubTemplate)
}

func (a *ServerController) refreshStatus() {
	a.lastStatus = a.serverService.GetStatus(a.lastStatus)
}

func (a *ServerController) getNewCert(c *gin.Context) {
	cert, err := a.serverService.GetNewCert()
	if err != nil {
		jsonMsg(c, "Error generating self-signed certificate", err)
		return
	}
	jsonObj(c, cert, nil)
}

func (a *ServerController) getNewmldsa65(c *gin.Context) {
	cert, err := a.serverService.GetNewmldsa65()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.getNewmldsa65Error"), err)
		return
	}
	jsonObj(c, cert, nil)
}

func (a *ServerController) startTask() {
	webServer := global.GetWebServer()
	c := webServer.GetCron()
	c.AddFunc("@every 2s", func() {
		now := time.Now()
		if now.Sub(a.lastGetStatusTime) > time.Minute*3 {
			return
		}
		a.refreshStatus()
	})
}

func (a *ServerController) status(c *gin.Context) {
	a.lastGetStatusTime = time.Now()

	jsonObj(c, a.lastStatus, nil)
}

func (a *ServerController) getXrayMetrics(c *gin.Context) {
	metrics, err := a.serverService.GetXrayMetrics()
	if err != nil {
		jsonMsg(c, "Failed to retrieve Xray metrics", err)
		return
	}
	jsonObj(c, metrics, nil)
}

func (a *ServerController) getXrayVersion(c *gin.Context) {
	now := time.Now()
	if now.Sub(a.lastGetVersionsTime) <= time.Minute {
		jsonObj(c, a.lastVersions, nil)
		return
	}

	versions, err := a.serverService.GetXrayVersions()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "getVersion"), err)
		return
	}

	a.lastVersions = versions
	a.lastGetVersionsTime = time.Now()

	jsonObj(c, versions, nil)
}

func (a *ServerController) installXray(c *gin.Context) {
	version := c.Param("version")
	err := a.serverService.UpdateXray(version)
	jsonMsg(c, I18nWeb(c, "install")+" xray", err)
}

func (a *ServerController) updateGeoFiles(c *gin.Context) {
	err := a.serverService.UpdateGeoFiles()
	jsonMsg(c, "update geo files", err)
}

func (a *ServerController) stopXrayService(c *gin.Context) {
	a.lastGetStatusTime = time.Now()
	err := a.serverService.StopXrayService()
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonMsg(c, "Xray stopped", err)
}

func (a *ServerController) restartXrayService(c *gin.Context) {
	err := a.serverService.RestartXrayService()
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonMsg(c, "Xray restarted", err)
}

func (a *ServerController) installPanel(c *gin.Context) {
	a.serverService.UpdatePanel("")
	jsonMsg(c, I18nWeb(c, "pages.index.installingPanel"), nil)
}

func (a *ServerController) setTunnel(c *gin.Context) {
	a.serverService.ApplyTunnel(c.Param("ip"), c.Param("port"), c.Param("user"), c.Param("password"))
	jsonMsg(c, I18nWeb(c, "pages.xray.tunnel.applied"), nil)
}

func (a *ServerController) getLogs(c *gin.Context) {
	count := c.Param("count")
	level := c.PostForm("level")
	syslog := c.PostForm("syslog")
	logs := a.serverService.GetLogs(count, level, syslog)
	jsonObj(c, logs, nil)
}

func (a *ServerController) getXrayLogs(c *gin.Context) {
	count := c.Param("count")
	filter := c.PostForm("filter")
	showDirect := c.PostForm("showDirect")
	showBlocked := c.PostForm("showBlocked")
	showProxy := c.PostForm("showProxy")

	var freedoms []string
	var blackholes []string

	//getting tags for freedom and blackhole outbounds
	config, err := a.settingService.GetDefaultXrayConfig()
	if err == nil && config != nil {
		if cfgMap, ok := config.(map[string]interface{}); ok {
			if outbounds, ok := cfgMap["outbounds"].([]interface{}); ok {
				for _, outbound := range outbounds {
					if obMap, ok := outbound.(map[string]interface{}); ok {
						switch obMap["protocol"] {
						case "freedom":
							if tag, ok := obMap["tag"].(string); ok {
								freedoms = append(freedoms, tag)
							}
						case "blackhole":
							if tag, ok := obMap["tag"].(string); ok {
								blackholes = append(blackholes, tag)
							}
						}
					}
				}
			}
		}
	}

	if len(freedoms) == 0 {
		freedoms = []string{"direct"}
	}
	if len(blackholes) == 0 {
		blackholes = []string{"blocked"}
	}

	logs := a.serverService.GetXrayLogs(count, filter, showDirect, showBlocked, showProxy, freedoms, blackholes)
	jsonObj(c, logs, nil)
}

func (a *ServerController) getConfigJson(c *gin.Context) {
	configJson, err := a.serverService.GetConfigJson()
	if err != nil {
		jsonMsg(c, "get config.json", err)
		return
	}
	jsonObj(c, configJson, nil)
}

func (a *ServerController) getDb(c *gin.Context) {
	db, err := a.serverService.GetDb()
	if err != nil {
		jsonMsg(c, "get Database", err)
		return
	}

	filename := "x-ui.db"

	if !isValidFilename(filename) {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("invalid filename"))
		return
	}

	// Set the headers for the response
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename="+filename)

	// Write the file contents to the response
	c.Writer.Write(db)
}

func isValidFilename(filename string) bool {
	// Validate that the filename only contains allowed characters
	return filenameRegex.MatchString(filename)
}

func (a *ServerController) importDB(c *gin.Context) {
	// Get the file from the request body
	file, _, err := c.Request.FormFile("db")
	if err != nil {
		jsonMsg(c, "Error reading db file", err)
		return
	}
	defer file.Close()
	// Always restart Xray before return
	defer a.serverService.RestartXrayService()
	defer func() {
		a.lastGetStatusTime = time.Now()
	}()
	// Import it
	err = a.serverService.ImportDB(file)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonObj(c, "Import DB", nil)
}

func (a *ServerController) getNewX25519Cert(c *gin.Context) {
	cert, err := a.serverService.GetNewX25519Cert()
	if err != nil {
		jsonMsg(c, "get x25519 certificate", err)
		return
	}
	jsonObj(c, cert, nil)
}
func (a *ServerController) getNewEchCert(c *gin.Context) {
	sni := c.PostForm("sni")
	cert, err := a.serverService.GetNewEchCert(sni)
	if err != nil {
		jsonMsg(c, "get ech certificate", err)
		return
	}
	jsonObj(c, cert, nil)
}

func (a *ServerController) getNewVlessEnc(c *gin.Context) {
	out, err := a.serverService.GetNewVlessEnc()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.getNewVlessEncError"), err)
		return
	}
	jsonObj(c, out, nil)
}

func (a *ServerController) getNewUUID(c *gin.Context) {
	uuidResp, err := a.serverService.GetNewUUID()
	if err != nil {
		jsonMsg(c, "Failed to generate UUID", err)
		return
	}

	jsonObj(c, uuidResp, nil)
}

func (a *ServerController) getNewmlkem768(c *gin.Context) {
	out, err := a.serverService.GetNewmlkem768()
	if err != nil {
		jsonMsg(c, "Failed to generate mlkem768 keys", err)
		return
	}
	jsonObj(c, out, nil)
}

func (a *ServerController) getSubTemplates(c *gin.Context) {
	res, err := a.serverService.GetSubTemplates()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.server.loadError"), err)
		return
	}
	jsonObj(c, res, nil)
}

func (a *ServerController) applySubTemplate(c *gin.Context) {
	type form struct {
		URL string `json:"url" form:"url"`
	}
	var f form
	if err := c.ShouldBind(&f); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.server.loadError"), err)
		return
	}
	if !isAllowedSubTemplateURL(f.URL) {
		jsonMsg(c, I18nWeb(c, "pages.server.loadError"), fmt.Errorf("%s", I18nWeb(c, "pages.settings.subTemplateModalAllowedUrlsOnly")))
		return
	}
	err := a.serverService.ApplySubTemplateFromGithub(f.URL)
	if err != nil {
		jsonMsg(c, err.Error(), nil)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.settings.subTemplateModalDownloadSuccess"), nil)
}

// isAllowedSubTemplateURL restricts sub-template downloads to GitHub hosts only,
// preventing SSRF via arbitrary or redirect-controlled destinations.
func isAllowedSubTemplateURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "github.com" || host == "raw.githubusercontent.com"
}

func (a *ServerController) resetSubTemplate(c *gin.Context) {
	err := a.serverService.ResetSubTemplate()
	if err != nil {
		jsonMsg(c, err.Error(), nil)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.settings.subTemplateModalResetSuccessMsg"), nil)
}
