package controller

import (
	"net"
	"net/http"
	"strings"

	"x-ui/config"
	"x-ui/internal/logger"
	"x-ui/internal/web/entity"
	"x-ui/internal/web/session"

	"github.com/gin-gonic/gin"
)

func getRemoteIp(c *gin.Context) string {
	value := c.GetHeader("X-Real-IP")
	if value != "" {
		return value
	}
	value = c.GetHeader("X-Forwarded-For")
	if value != "" {
		ips := strings.Split(value, ",")
		return ips[0]
	}
	addr := c.Request.RemoteAddr
	ip, _, _ := net.SplitHostPort(addr)
	return ip
}

func jsonMsg(c *gin.Context, msg string, err error) {
	jsonMsgObj(c, msg, nil, err)
}

func jsonObj(c *gin.Context, obj interface{}, err error) {
	jsonMsgObj(c, "", obj, err)
}

func jsonMsgObj(c *gin.Context, msg string, obj interface{}, err error) {
	m := entity.Msg{
		Obj: obj,
	}
	if err == nil {
		m.Success = true
		if msg != "" {
			m.Msg = msg + " " + I18nWeb(c, "success")
		}
	} else {
		m.Success = false
		m.Msg = msg + " " + I18nWeb(c, "fail") + ": " + err.Error()
		logger.Warning(msg+" "+I18nWeb(c, "fail")+": ", err)
	}
	c.JSON(http.StatusOK, m)
}

func pureJsonMsg(c *gin.Context, statusCode int, success bool, msg string) {
	c.JSON(statusCode, entity.Msg{
		Success: success,
		Msg:     msg,
	})
}

func html(c *gin.Context, name string, title string, data gin.H) {
	htmlStatus(c, http.StatusOK, name, title, data)
}

func htmlStatus(c *gin.Context, status int, name string, title string, data gin.H) {
	if data == nil {
		data = gin.H{}
	}
	data["title"] = title
	data["user"] = session.GetLoginUser(c)
	host := c.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = c.GetHeader("X-Real-IP")
	}
	if host == "" {
		var err error
		host, _, err = net.SplitHostPort(c.Request.Host)
		if err != nil {
			host = c.Request.Host
		}
	}
	data["host"] = host
	data["request_uri"] = c.Request.RequestURI
	data["base_path"] = c.GetString("base_path")
	c.HTML(status, name, getContext(data))
}

func render403(c *gin.Context, reasonKey string) {
	reason := ""
	if reasonKey != "" {
		reason = I18nWeb(c, reasonKey)
		if reason == "" {
			reason = reasonKey
		}
	} else {
		reason = I18nWeb(c, "pages.forbidden.adminRequired")
	}
	htmlStatus(c, http.StatusForbidden, "403.html", "pages.forbidden.title", gin.H{
		"reason": reason,
	})
	c.Abort()
}

func Render404(c *gin.Context) {
	basePath := c.GetString("base_path")
	path := c.Request.URL.Path
	if isAjax(c) || strings.Contains(c.GetHeader("Accept"), "application/json") ||
		(basePath != "" && (strings.HasPrefix(path, basePath+"api/") || strings.HasPrefix(path, basePath+"server/"))) ||
		strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/server/") {
		pureJsonMsg(c, http.StatusNotFound, false, I18nWeb(c, "pages.notFound.title"))
		c.Abort()
		return
	}
	htmlStatus(c, http.StatusNotFound, "404.html", "pages.notFound.title", gin.H{})
	c.Abort()
}

func render404(c *gin.Context) {
	Render404(c)
}

func getContext(h gin.H) gin.H {
	a := gin.H{
		"cur_ver": config.GetVersion(),
	}
	for key, value := range h {
		a[key] = value
	}
	return a
}

func isAjax(c *gin.Context) bool {
	return c.GetHeader("X-Requested-With") == "XMLHttpRequest"
}
