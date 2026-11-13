package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"x-ui/internal/database"
	"x-ui/internal/database/model"
	"x-ui/internal/web/service"
	"x-ui/internal/web/session"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type InboundController struct {
	inboundService service.InboundService
	xrayService    service.XrayService
}

func NewInboundController(g *gin.RouterGroup) *InboundController {
	a := &InboundController{}
	a.initRouter(g)
	return a
}

func (a *InboundController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.getInbounds)
	g.GET("/get/:id", a.getInbound)
	g.GET("/getClientTraffics/:email", a.getClientTraffics)
	g.GET("/getClientTrafficsById/:id", a.getClientTrafficsById)
	g.POST("/updateClientTraffic/:email", a.updateClientTraffic)
	g.POST("/add", a.addInbound)
	g.POST("/del/:id", a.delInbound)
	g.POST("/update/:id", a.updateInbound)
	g.POST("/clientIps/:email", a.getClientIps)
	g.POST("/clearClientIps/:email", a.clearClientIps)
	g.POST("/addClient", a.addInboundClient)
	g.POST("/:id/delClient/:clientId", a.delInboundClient)
	g.POST("/updateClient/:clientId", a.updateInboundClient)
	g.POST("/:id/resetClientTraffic/:email", a.resetClientTraffic)
	g.POST("/resetAllTraffics", a.resetAllTraffics)
	g.POST("/resetAllClientTraffics/:id", a.resetAllClientTraffics)
	g.POST("/delDepletedClients/:id", a.delDepletedClients)
	g.POST("/:id/delClientByEmail/:email", a.delInboundClientByEmail)
	g.POST("/import", a.importInbound)
	g.POST("/onlines", a.onlines)
	g.POST("/depleted", a.depleted)
	g.POST("/disabled", a.disabled)
	g.POST("/reorder", a.reorderInbounds)
}

func (a *InboundController) getInbounds(c *gin.Context) {
	user := session.GetLoginUser(c)
	var inbounds []*model.Inbound
	var err error

	if user == nil || user.IsAdmin() || user.Role == "viewer" || user.AllowedInbounds == "*" || user.AllowedInbounds == "" {
		inbounds, err = a.inboundService.GetAllInbounds()
	} else {
		all, e := a.inboundService.GetAllInbounds()
		if e != nil {
			err = e
		} else {
			for _, inb := range all {
				if user.CanAccessInbound(inb.Id) {
					inbounds = append(inbounds, inb)
				}
			}
		}
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			jsonObj(c, []*model.Inbound{}, nil)
			return
		}
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	if inbounds == nil {
		inbounds = []*model.Inbound{}
	}
	jsonObj(c, inbounds, nil)
}

func (a *InboundController) getInbound(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() && !user.CanAccessInbound(id) {
		pureJsonMsg(c, http.StatusForbidden, false, "Access denied to this inbound")
		return
	}
	inbound, err := a.inboundService.GetInbound(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			jsonObj(c, nil, nil)
			return
		}
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	jsonObj(c, inbound, nil)
}

func (a *InboundController) getClientTraffics(c *gin.Context) {
	email := c.Param("email")
	clientTraffics, err := a.inboundService.GetClientTrafficByEmail(email)
	if err != nil {
		jsonMsg(c, "Error getting traffics", err)
		return
	}
	jsonObj(c, clientTraffics, nil)
}

func (a *InboundController) getClientTrafficsById(c *gin.Context) {
	id := c.Param("id")
	clientTraffics, err := a.inboundService.GetClientTrafficByID(id)
	if err != nil {
		jsonMsg(c, "Error getting traffics", err)
		return
	}
	jsonObj(c, clientTraffics, nil)
}

func (a *InboundController) addInbound(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		if user.Role == "viewer" || !user.HasPermission("inbounds:create") {
			pureJsonMsg(c, http.StatusForbidden, false, "Permission denied: cannot create inbounds")
			return
		}
	}
	inbound := &model.Inbound{}
	err := c.ShouldBind(inbound)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.create"), err)
		return
	}
	if user != nil {
		inbound.UserId = user.Id
	}
	if inbound.Listen == "" || inbound.Listen == "0.0.0.0" || inbound.Listen == "::" || inbound.Listen == "::0" {
		inbound.Tag = fmt.Sprintf("inbound-%v", inbound.Port)
	} else {
		inbound.Tag = fmt.Sprintf("inbound-%v:%v", inbound.Listen, inbound.Port)
	}

	needRestart := false
	inbound, needRestart, err = a.inboundService.AddInbound(inbound)
	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.create"), inbound, err)
	if err == nil {
		if user != nil && !user.IsAdmin() && user.AllowedInbounds != "*" && user.AllowedInbounds != "" {
			var ids []int
			if jsonErr := json.Unmarshal([]byte(user.AllowedInbounds), &ids); jsonErr == nil {
				ids = append(ids, inbound.Id)
				b, _ := json.Marshal(ids)
				user.AllowedInbounds = string(b)
			} else {
				user.AllowedInbounds = fmt.Sprintf("%s, %d", user.AllowedInbounds, inbound.Id)
			}
			database.GetDB().Model(&model.User{}).Where("id = ?", user.Id).Update("allowed_inbounds", user.AllowedInbounds)
			session.SetLoginUser(c, user)
		}
		if needRestart {
			a.xrayService.SetToNeedRestart()
		}
	}
}

func (a *InboundController) delInbound(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "delete"), err)
		return
	}
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		if user.Role == "viewer" || !user.HasPermission("inbounds:delete") || !user.CanAccessInbound(id) {
			pureJsonMsg(c, http.StatusForbidden, false, "Permission denied to delete inbound")
			return
		}
	}
	needRestart := true
	needRestart, err = a.inboundService.DelInbound(id)
	jsonMsgObj(c, I18nWeb(c, "delete"), id, err)
	if err == nil && needRestart {
		a.xrayService.SetToNeedRestart()
	}
}

func (a *InboundController) updateInbound(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.update"), err)
		return
	}
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		if user.Role == "viewer" || (!user.HasPermission("inbounds:edit") && !user.HasPermission("inbounds:update")) || !user.CanAccessInbound(id) {
			pureJsonMsg(c, http.StatusForbidden, false, "Permission denied to update inbound")
			return
		}
	}
	inbound := &model.Inbound{
		Id: id,
	}
	err = c.ShouldBind(inbound)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.update"), err)
		return
	}
	needRestart := true
	inbound, needRestart, err = a.inboundService.UpdateInbound(inbound)
	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.update"), inbound, err)
	if err == nil && needRestart {
		a.xrayService.SetToNeedRestart()
	}
}

func (a *InboundController) getClientIps(c *gin.Context) {
	email := c.Param("email")

	ips, err := a.inboundService.GetInboundClientIps(email)
	if err != nil || ips == "" {
		jsonObj(c, "No IP Record", nil)
		return
	}

	jsonObj(c, ips, nil)
}

func (a *InboundController) clearClientIps(c *gin.Context) {
	email := c.Param("email")

	err := a.inboundService.ClearClientIps(email)
	if err != nil {
		jsonMsg(c, "Update", err)
		return
	}
	jsonMsg(c, "Log Cleared", nil)
}

func (a *InboundController) addInboundClient(c *gin.Context) {
	data := &model.Inbound{}
	err := c.ShouldBind(data)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.update"), err)
		return
	}
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		if user.Role == "viewer" || !user.HasPermission("clients:create") || !user.CanAccessInbound(data.Id) {
			pureJsonMsg(c, http.StatusForbidden, false, "Permission denied to add client to this inbound")
			return
		}
	}

	needRestart := true

	needRestart, err = a.inboundService.AddInboundClient(data)
	if err != nil {
		jsonMsg(c, "Something went wrong!", err)
		return
	}
	jsonMsg(c, "Client(s) added", nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
		go func() {
			_ = a.xrayService.RestartXray(true)
		}()
	}
}

func (a *InboundController) delInboundClient(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.update"), err)
		return
	}
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		if user.Role == "viewer" || !user.HasPermission("clients:delete") || !user.CanAccessInbound(id) {
			pureJsonMsg(c, http.StatusForbidden, false, "Permission denied to delete client from this inbound")
			return
		}
	}
	clientId := c.Param("clientId")

	needRestart := true

	needRestart, err = a.inboundService.DelInboundClient(id, clientId)
	if err != nil {
		jsonMsg(c, "Something went wrong!", err)
		return
	}
	jsonMsg(c, "Client deleted", nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
		go func() {
			_ = a.xrayService.RestartXray(true)
		}()
	}
}

func (a *InboundController) updateInboundClient(c *gin.Context) {
	clientId := c.Param("clientId")

	inbound := &model.Inbound{}
	err := c.ShouldBind(inbound)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.update"), err)
		return
	}
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		if user.Role == "viewer" || (!user.HasPermission("clients:edit") && !user.HasPermission("clients:update")) || (inbound.Id > 0 && !user.CanAccessInbound(inbound.Id)) {
			pureJsonMsg(c, http.StatusForbidden, false, "Permission denied to update client")
			return
		}
	}

	needRestart := true

	needRestart, err = a.inboundService.UpdateInboundClient(inbound, clientId)
	if err != nil {
		jsonMsg(c, "Something went wrong!", err)
		return
	}
	jsonMsg(c, "Client updated", nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
		go func() {
			_ = a.xrayService.RestartXray(true)
		}()
	}
}

func (a *InboundController) resetClientTraffic(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.update"), err)
		return
	}
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		if !user.CanResetTraffic || user.Role == "viewer" || !user.CanAccessInbound(id) {
			pureJsonMsg(c, http.StatusForbidden, false, "Permission denied: traffic reset not allowed")
			return
		}
	}
	email := c.Param("email")

	needRestart, err := a.inboundService.ResetClientTraffic(id, email)
	if err != nil {
		jsonMsg(c, "Something went wrong!", err)
		return
	}
	jsonMsg(c, "Traffic has been reset", nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
		go func() {
			_ = a.xrayService.RestartXray(true)
		}()
	}
}

func (a *InboundController) resetAllTraffics(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		pureJsonMsg(c, http.StatusForbidden, false, "Permission denied: admin role required")
		return
	}
	err := a.inboundService.ResetAllTraffics()
	if err != nil {
		jsonMsg(c, "Something went wrong!", err)
		return
	} else {
		a.xrayService.SetToNeedRestart()
	}
	jsonMsg(c, "all traffic has been reset", nil)
}

func (a *InboundController) resetAllClientTraffics(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.update"), err)
		return
	}
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		if !user.CanResetTraffic || user.Role == "viewer" || !user.CanAccessInbound(id) {
			pureJsonMsg(c, http.StatusForbidden, false, "Permission denied: traffic reset not allowed")
			return
		}
	}

	err = a.inboundService.ResetAllClientTraffics(id)
	if err != nil {
		jsonMsg(c, "Something went wrong!", err)
		return
	} else {
		a.xrayService.SetToNeedRestart()
	}
	jsonMsg(c, "All traffic from the client has been reset.", nil)
}

func (a *InboundController) importInbound(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		if user.Role == "viewer" || !user.HasPermission("inbounds:create") {
			pureJsonMsg(c, http.StatusForbidden, false, "Permission denied to import inbounds")
			return
		}
	}
	inbound := &model.Inbound{}
	err := json.Unmarshal([]byte(c.PostForm("data")), inbound)
	if err != nil {
		jsonMsg(c, "Something went wrong!", err)
		return
	}
	inbound.Id = 0
	if user != nil {
		inbound.UserId = user.Id
	}
	if inbound.Listen == "" || inbound.Listen == "0.0.0.0" || inbound.Listen == "::" || inbound.Listen == "::0" {
		inbound.Tag = fmt.Sprintf("inbound-%v", inbound.Port)
	} else {
		inbound.Tag = fmt.Sprintf("inbound-%v:%v", inbound.Listen, inbound.Port)
	}

	for index := range inbound.ClientStats {
		inbound.ClientStats[index].Id = 0
		inbound.ClientStats[index].Enable = true
	}

	needRestart := false
	inbound, needRestart, err = a.inboundService.AddInbound(inbound)
	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.create"), inbound, err)
	if err == nil {
		if user != nil && !user.IsAdmin() && user.AllowedInbounds != "*" && user.AllowedInbounds != "" {
			var ids []int
			if jsonErr := json.Unmarshal([]byte(user.AllowedInbounds), &ids); jsonErr == nil {
				ids = append(ids, inbound.Id)
				b, _ := json.Marshal(ids)
				user.AllowedInbounds = string(b)
			} else {
				user.AllowedInbounds = fmt.Sprintf("%s, %d", user.AllowedInbounds, inbound.Id)
			}
			database.GetDB().Model(&model.User{}).Where("id = ?", user.Id).Update("allowed_inbounds", user.AllowedInbounds)
			session.SetLoginUser(c, user)
		}
		if needRestart {
			a.xrayService.SetToNeedRestart()
		}
	}
}

func (a *InboundController) delDepletedClients(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.update"), err)
		return
	}
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		if user.Role == "viewer" || !user.HasPermission("clients:delete") || !user.CanAccessInbound(id) {
			pureJsonMsg(c, http.StatusForbidden, false, "Permission denied to delete depleted clients")
			return
		}
	}
	err = a.inboundService.DelDepletedClients(id)
	if err != nil {
		jsonMsg(c, "Something went wrong!", err)
		return
	}
	jsonMsg(c, "All depleted clients are deleted", nil)
}

func (a *InboundController) onlines(c *gin.Context) {
	jsonObj(c, a.inboundService.GetOnlineClients(), nil)
}

func (a *InboundController) depleted(c *gin.Context) {
	depletedClients, err := a.inboundService.GetDepletedClients()
	if err != nil {
		jsonMsg(c, "Error getting depleted clients", err)
		return
	}
	jsonObj(c, depletedClients, nil)
}

func (a *InboundController) disabled(c *gin.Context) {
	disabledClients, err := a.inboundService.GetDisabledClients()
	if err != nil {
		jsonMsg(c, "Error getting disabled clients", err)
		return
	}
	jsonObj(c, disabledClients, nil)
}

func (a *InboundController) updateClientTraffic(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		if !user.CanResetTraffic || user.Role == "viewer" {
			pureJsonMsg(c, http.StatusForbidden, false, "Permission denied to modify client traffic")
			return
		}
	}
	email := c.Param("email")

	// Define the request structure for traffic update
	type TrafficUpdateRequest struct {
		Upload   int64 `json:"upload"`
		Download int64 `json:"download"`
	}

	var request TrafficUpdateRequest
	err := c.ShouldBindJSON(&request)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.update"), err)
		return
	}

	err = a.inboundService.UpdateClientTrafficByEmail(email, request.Upload, request.Download)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}

	jsonMsg(c, I18nWeb(c, "pages.inbounds.update"), nil)
}

func (a *InboundController) delInboundClientByEmail(c *gin.Context) {
	inboundId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid inbound ID", err)
		return
	}
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		if user.Role == "viewer" || !user.HasPermission("clients:delete") || !user.CanAccessInbound(inboundId) {
			pureJsonMsg(c, http.StatusForbidden, false, "Permission denied to delete client by email")
			return
		}
	}

	email := c.Param("email")
	needRestart, err := a.inboundService.DelInboundClientByEmail(inboundId, email)
	if err != nil {
		jsonMsg(c, "Failed to delete client by email", err)
		return
	}

	jsonMsg(c, "Client deleted successfully", nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
		go func() {
			_ = a.xrayService.RestartXray(true)
		}()
	}
}

func (a *InboundController) reorderInbounds(c *gin.Context) {
	var body struct {
		Ids []int `json:"ids"`
	}

	// Try JSON bind first
	if err := c.ShouldBindJSON(&body); err != nil {
		// Fallback: read raw body and attempt to parse url-encoded or plain JSON
		raw, _ := c.GetRawData()
		rawStr := strings.TrimSpace(string(raw))
		// Try url-encoded parsing (ids=1&ids=2 or ids=1,2)
		if strings.Contains(rawStr, "=") {
			vals, err := url.ParseQuery(rawStr)
			if err == nil {
				var ids []int
				if arr, ok := vals["ids"]; ok && len(arr) > 0 {
					for _, entry := range arr {
						// entry could be "1" or "1,2,3"
						parts := strings.Split(entry, ",")
						for _, p := range parts {
							p = strings.TrimSpace(p)
							if p == "" {
								continue
							}
							if n, perr := strconv.Atoi(p); perr == nil {
								ids = append(ids, n)
							}
						}
					}
				} else if s := vals.Get("ids"); s != "" {
					parts := strings.Split(s, ",")
					for _, p := range parts {
						p = strings.TrimSpace(p)
						if p == "" {
							continue
						}
						if n, perr := strconv.Atoi(p); perr == nil {
							ids = append(ids, n)
						}
					}
				}
				body.Ids = ids
			}
		} else {
			// Try parsing as JSON (in case GetRawData consumed earlier ShouldBindJSON)
			_ = json.Unmarshal(raw, &body)
		}
	}
	// If body still empty, error
	if len(body.Ids) == 0 {
		jsonMsg(c, "Reorder Inbounds", fmt.Errorf("ids cannot be empty or invalid: %v", body.Ids))
		return
	}

	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		if user.Role == "viewer" {
			pureJsonMsg(c, http.StatusForbidden, false, "Permission denied: viewer cannot reorder inbounds")
			return
		}
		for _, id := range body.Ids {
			if !user.CanAccessInbound(id) {
				pureJsonMsg(c, http.StatusForbidden, false, fmt.Sprintf("Permission denied: inbound %d is not in your allowed scope", id))
				return
			}
		}
	}
	// load user's inbounds ids
	inbounds, err := a.inboundService.GetInbounds(user.Id)
	if err != nil {
		jsonMsg(c, "Reorder Inbounds", err)
		return
	}
	validIds := make(map[int]bool)
	for _, ib := range inbounds {
		validIds[ib.Id] = true
	}
	for _, id := range body.Ids {
		if !validIds[id] {
			jsonMsg(c, "Reorder Inbounds", fmt.Errorf("inbound id %d not found or not owned by user", id))
			return
		}
	}

	err = a.inboundService.ReorderInbounds(body.Ids)
	if err != nil {
		jsonMsg(c, "Reorder Inbounds", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "msg": I18nWeb(c, "pages.inbounds.reordered")})
}
