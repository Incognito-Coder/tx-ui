package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"x-ui/internal/database/model"
	"x-ui/internal/logger"
	"x-ui/internal/web/service"
	"x-ui/internal/web/session"

	"github.com/gin-gonic/gin"
)

type NodeClientController struct {
	nodeClientService service.NodeClientService
	xrayService       service.XrayService
}

func NewNodeClientController(g *gin.RouterGroup) *NodeClientController {
	a := &NodeClientController{}
	a.initRouter(g)
	return a
}

func (a *NodeClientController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.list)
	g.GET("/get/:id", a.getOne)
	g.POST("/create", a.create)
	g.POST("/bulkCreate", a.bulkCreate)
	g.POST("/update/:id", a.update)
	g.POST("/del/:id", a.del)
	g.POST("/bulkDel", a.bulkDel)
	g.GET("/:id/links", a.getLinks)
	g.POST("/:id/addLink", a.addLink)
	g.POST("/:id/setLinks", a.setLinks)
	g.POST("/bulkSetLinks", a.bulkSetLinks)
	g.POST("/:id/removeLink/:inboundId", a.removeLink)
	g.GET("/:id/traffic", a.getTraffic)
	g.POST("/:id/resetTraffic", a.resetTraffic)
	g.POST("/resetAllTraffics", a.resetAllTraffics)
	g.POST("/delDepleted", a.delDepleted)
	g.GET("/inboundLinkCounts", a.inboundLinkCounts)
	g.POST("/:id/toggle", a.toggle)
	g.GET("/:id/devices", a.getDevices)
	g.POST("/:id/clearDevices", a.clearDevices)
}

func (a *NodeClientController) list(c *gin.Context) {
	clients, err := a.nodeClientService.GetAllWithDetails()
	if err != nil {
		jsonMsg(c, "Failed to get clients", err)
		return
	}
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() && user.AllowedInbounds != "*" && user.AllowedInbounds != "" {
		filtered := make([]*service.ClientDetail, 0)
		for _, cd := range clients {
			allowedLinks := make([]model.NodeClientLink, 0)
			for _, link := range cd.Links {
				if user.CanAccessInbound(link.InboundId) {
					allowedLinks = append(allowedLinks, link)
				}
			}
			if len(allowedLinks) > 0 {
				cd.Links = allowedLinks
				filtered = append(filtered, cd)
			}
		}
		clients = filtered
	}
	jsonObj(c, clients, nil)
}

func (a *NodeClientController) getOne(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid client ID", err)
		return
	}

	nc, err := a.nodeClientService.GetByID(id)
	if err != nil {
		jsonMsg(c, "Client not found", err)
		c.Status(http.StatusNotFound)
		return
	}
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() && user.AllowedInbounds != "*" && user.AllowedInbounds != "" {
		links, _ := a.nodeClientService.GetLinks(nc.Id)
		canAccess := false
		for _, l := range links {
			if user.CanAccessInbound(l.InboundId) {
				canAccess = true
				break
			}
		}
		if !canAccess {
			pureJsonMsg(c, http.StatusForbidden, false, "Access denied to this client")
			return
		}
	}
	jsonObj(c, nc, nil)
}

func (a *NodeClientController) create(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		if user.Role == "viewer" || !user.HasPermission("clients:create") {
			pureJsonMsg(c, http.StatusForbidden, false, "Permission denied to create client")
			return
		}
	}

	var req struct {
		model.NodeClient
		InboundIds []int `json:"inboundIds" form:"inboundIds"`
	}

	raw, _ := c.GetRawData()
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &req)
	}
	if req.NodeClient.Email == "" {
		c.Request.Body = io.NopCloser(bytes.NewBuffer(raw))
		_ = c.ShouldBind(&req)
	}
	if len(req.InboundIds) == 0 {
		formIds := c.PostFormArray("inboundIds")
		if len(formIds) == 0 {
			formIds = c.PostFormArray("inboundIds[]")
		}
		for _, fid := range formIds {
			if n, perr := strconv.Atoi(fid); perr == nil {
				req.InboundIds = append(req.InboundIds, n)
			}
		}
	}

	if user != nil && !user.IsAdmin() {
		for _, inboundId := range req.InboundIds {
			if !user.CanAccessInbound(inboundId) {
				pureJsonMsg(c, http.StatusForbidden, false, fmt.Sprintf("Permission denied: inbound %d is outside your allowed scope", inboundId))
				return
			}
		}
	}

	nc := &req.NodeClient
	err := a.nodeClientService.Create(nc)
	if err != nil {
		jsonMsg(c, "Failed to create client: "+err.Error(), err)
		return
	}

	needRestart := false
	if len(req.InboundIds) > 0 {
		links := make([]service.NodeClientLinkInput, len(req.InboundIds))
		for i, inboundId := range req.InboundIds {
			links[i] = service.NodeClientLinkInput{
				InboundId: inboundId,
				Flow:      nc.Flow,
			}
		}
		needRestart, _ = a.nodeClientService.SetLinks(nc.Id, links)
	}

	if needRestart {
		a.xrayService.SetToNeedRestart()
		go func() {
			_ = a.xrayService.RestartXray(true)
		}()
	}
	jsonMsg(c, "Client created", nil)
}

func (a *NodeClientController) bulkCreate(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		if user.Role == "viewer" || !user.HasPermission("clients:create") {
			pureJsonMsg(c, http.StatusForbidden, false, "Permission denied to bulk create clients")
			return
		}
	}

	var req struct {
		Clients    []model.NodeClient `json:"clients" form:"clients"`
		InboundIds []int              `json:"inboundIds" form:"inboundIds"`
	}

	raw, _ := c.GetRawData()
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &req)
	}
	if len(req.Clients) == 0 {
		c.Request.Body = io.NopCloser(bytes.NewBuffer(raw))
		_ = c.ShouldBind(&req)
	}
	if len(req.Clients) == 0 {
		jsonMsg(c, "No clients provided", nil)
		return
	}
	if len(req.InboundIds) == 0 {
		formIds := c.PostFormArray("inboundIds")
		if len(formIds) == 0 {
			formIds = c.PostFormArray("inboundIds[]")
		}
		for _, fid := range formIds {
			if n, perr := strconv.Atoi(fid); perr == nil {
				req.InboundIds = append(req.InboundIds, n)
			}
		}
	}

	if user != nil && !user.IsAdmin() {
		for _, inboundId := range req.InboundIds {
			if !user.CanAccessInbound(inboundId) {
				pureJsonMsg(c, http.StatusForbidden, false, fmt.Sprintf("Permission denied: inbound %d is outside your allowed scope", inboundId))
				return
			}
		}
	}

	needRestart, err := a.nodeClientService.BulkCreate(req.Clients, req.InboundIds)
	if err != nil {
		jsonMsg(c, "Bulk create failed: "+err.Error(), err)
		return
	}

	if needRestart {
		a.xrayService.SetToNeedRestart()
		go func() {
			_ = a.xrayService.RestartXray(true)
		}()
	}
	jsonMsg(c, fmt.Sprintf("Successfully created %d clients", len(req.Clients)), nil)
}

func (a *NodeClientController) setLinks(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid client ID", err)
		return
	}
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		if user.Role == "viewer" || (!user.HasPermission("clients:edit") && !user.HasPermission("clients:update")) {
			pureJsonMsg(c, http.StatusForbidden, false, "Permission denied to modify client links")
			return
		}
	}

	var req struct {
		Links []service.NodeClientLinkInput `json:"links" form:"links"`
	}

	raw, _ := c.GetRawData()
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &req)
	}
	if len(req.Links) == 0 {
		c.Request.Body = io.NopCloser(bytes.NewBuffer(raw))
		_ = c.ShouldBind(&req)
		if len(req.Links) == 0 {
			if linksStr := c.PostForm("links"); linksStr != "" {
				_ = json.Unmarshal([]byte(linksStr), &req.Links)
			}
		}
	}

	if user != nil && !user.IsAdmin() {
		for _, link := range req.Links {
			if !user.CanAccessInbound(link.InboundId) {
				pureJsonMsg(c, http.StatusForbidden, false, fmt.Sprintf("Permission denied: inbound %d is outside your allowed scope", link.InboundId))
				return
			}
		}
	}

	needRestart, err := a.nodeClientService.SetLinks(id, req.Links)
	if err != nil {
		jsonMsg(c, "Failed to set links: "+err.Error(), err)
		return
	}

	if needRestart {
		a.xrayService.SetToNeedRestart()
		go func() {
			_ = a.xrayService.RestartXray(true)
		}()
	}
	jsonMsg(c, "Links updated", nil)
}

func (a *NodeClientController) bulkSetLinks(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		if user.Role == "viewer" || (!user.HasPermission("clients:edit") && !user.HasPermission("clients:update")) {
			pureJsonMsg(c, http.StatusForbidden, false, "Permission denied to modify client links")
			return
		}
	}

	var req struct {
		ClientIds  []int  `json:"clientIds" form:"clientIds"`
		InboundIds []int  `json:"inboundIds" form:"inboundIds"`
		Action     string `json:"action" form:"action"`
		Flow       string `json:"flow" form:"flow"`
	}

	raw, _ := c.GetRawData()
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &req)
	}
	if len(req.ClientIds) == 0 {
		c.Request.Body = io.NopCloser(bytes.NewBuffer(raw))
		_ = c.ShouldBind(&req)
	}
	if len(req.ClientIds) == 0 {
		formIds := c.PostFormArray("clientIds")
		if len(formIds) == 0 {
			formIds = c.PostFormArray("clientIds[]")
		}
		for _, fid := range formIds {
			if n, perr := strconv.Atoi(fid); perr == nil {
				req.ClientIds = append(req.ClientIds, n)
			}
		}
	}
	if len(req.ClientIds) == 0 {
		jsonMsg(c, "No clients selected", nil)
		return
	}
	if len(req.InboundIds) == 0 && req.Action != "set" && req.Action != "replace" {
		formIds := c.PostFormArray("inboundIds")
		if len(formIds) == 0 {
			formIds = c.PostFormArray("inboundIds[]")
		}
		for _, fid := range formIds {
			if n, perr := strconv.Atoi(fid); perr == nil {
				req.InboundIds = append(req.InboundIds, n)
			}
		}
	}

	if user != nil && !user.IsAdmin() {
		for _, inboundId := range req.InboundIds {
			if !user.CanAccessInbound(inboundId) {
				pureJsonMsg(c, http.StatusForbidden, false, fmt.Sprintf("Permission denied: inbound %d is outside your allowed scope", inboundId))
				return
			}
		}
	}

	needRestart, err := a.nodeClientService.BulkSetLinks(req.ClientIds, req.InboundIds, req.Action, req.Flow)
	if err != nil {
		jsonMsg(c, "Bulk update links failed: "+err.Error(), err)
		return
	}

	if needRestart {
		a.xrayService.SetToNeedRestart()
		go func() {
			_ = a.xrayService.RestartXray(true)
		}()
	}
	jsonMsg(c, fmt.Sprintf("Successfully updated links for %d clients", len(req.ClientIds)), nil)
}

func (a *NodeClientController) resetAllTraffics(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		pureJsonMsg(c, http.StatusForbidden, false, "Permission denied: admin role required to reset all traffics")
		return
	}
	err := a.nodeClientService.ResetAllTraffics()
	if err != nil {
		jsonMsg(c, "Failed to reset traffic", err)
		return
	}
	a.xrayService.SetToNeedRestart()
	go func() {
		_ = a.xrayService.RestartXray(true)
	}()
	jsonMsg(c, "All client traffic reset", nil)
}

func (a *NodeClientController) delDepleted(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		if user.Role == "viewer" || !user.HasPermission("clients:delete") {
			pureJsonMsg(c, http.StatusForbidden, false, "Permission denied: cannot delete depleted clients")
			return
		}
	}
	needRestart, err := a.nodeClientService.DeleteDepleted()
	if err != nil {
		jsonMsg(c, "Failed to delete depleted clients", err)
		return
	}
	if needRestart {
		a.xrayService.SetToNeedRestart()
		go func() {
			_ = a.xrayService.RestartXray(true)
		}()
	}
	jsonMsg(c, "Depleted clients deleted", nil)
}

func (a *NodeClientController) inboundLinkCounts(c *gin.Context) {
	counts, err := a.nodeClientService.GetLinkedInboundsCounts()
	if err != nil {
		jsonMsg(c, "Failed to get linked counts", err)
		return
	}
	jsonObj(c, counts, nil)
}

func (a *NodeClientController) update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid node client ID", err)
		return
	}
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		if user.Role == "viewer" || (!user.HasPermission("clients:edit") && !user.HasPermission("clients:update")) {
			pureJsonMsg(c, http.StatusForbidden, false, "Permission denied to update client")
			return
		}
	}

	existing, err := a.nodeClientService.GetByID(id)
	if err != nil || existing == nil {
		jsonMsg(c, "Client not found", err)
		c.Status(http.StatusNotFound)
		return
	}

	var req struct {
		model.NodeClient
		InboundIds *[]int `json:"inboundIds" form:"inboundIds"`
	}

	raw, _ := c.GetRawData()
	var rawMap map[string]interface{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &rawMap)
		_ = json.Unmarshal(raw, &req)
	}
	if req.NodeClient.Email == "" {
		c.Request.Body = io.NopCloser(bytes.NewBuffer(raw))
		_ = c.ShouldBind(&req)
	}

	mergedNC := *existing
	if rawMap != nil {
		if val, ok := rawMap["expiryTime"]; ok {
			mergedNC.ExpiryTime = toInt64(val)
		} else if val, ok := rawMap["expiry_time"]; ok {
			mergedNC.ExpiryTime = toInt64(val)
		}
		if val, ok := rawMap["totalGB"]; ok {
			mergedNC.TotalGB = toInt64(val)
		} else if val, ok := rawMap["total_gb"]; ok {
			mergedNC.TotalGB = toInt64(val)
		}
		if val, ok := rawMap["email"]; ok && val != nil {
			if s, ok := val.(string); ok && strings.TrimSpace(s) != "" {
				mergedNC.Email = strings.TrimSpace(s)
			}
		}
		if val, ok := rawMap["enable"]; ok && val != nil {
			if b, ok := val.(bool); ok {
				mergedNC.Enable = b
			}
		}
		if val, ok := rawMap["reset"]; ok {
			mergedNC.Reset = toInt(val)
		}
		if val, ok := rawMap["subId"]; ok && val != nil {
			mergedNC.SubID = fmt.Sprintf("%v", val)
		} else if val, ok := rawMap["sub_id"]; ok && val != nil {
			mergedNC.SubID = fmt.Sprintf("%v", val)
		}
		if val, ok := rawMap["limitIp"]; ok {
			mergedNC.LimitIP = toInt(val)
		} else if val, ok := rawMap["limit_ip"]; ok {
			mergedNC.LimitIP = toInt(val)
		}
		if val, ok := rawMap["tgId"]; ok {
			mergedNC.TgID = toInt64(val)
		} else if val, ok := rawMap["tg_id"]; ok {
			mergedNC.TgID = toInt64(val)
		}
		if val, ok := rawMap["comment"]; ok && val != nil {
			mergedNC.Comment = fmt.Sprintf("%v", val)
		}
		if val, ok := rawMap["flow"]; ok && val != nil {
			mergedNC.Flow = fmt.Sprintf("%v", val)
		}
		if val, ok := rawMap["uuid"]; ok && val != nil {
			mergedNC.UUID = fmt.Sprintf("%v", val)
		}
		if val, ok := rawMap["password"]; ok && val != nil {
			mergedNC.Password = fmt.Sprintf("%v", val)
		}
		if val, ok := rawMap["ssPassword"]; ok && val != nil {
			mergedNC.SSPassword = fmt.Sprintf("%v", val)
		} else if val, ok := rawMap["ss_password"]; ok && val != nil {
			mergedNC.SSPassword = fmt.Sprintf("%v", val)
		}
		if val, ok := rawMap["masquePass"]; ok && val != nil {
			mergedNC.MasquePass = fmt.Sprintf("%v", val)
		} else if val, ok := rawMap["masque_pass"]; ok && val != nil {
			mergedNC.MasquePass = fmt.Sprintf("%v", val)
		}
		if val, ok := rawMap["auth"]; ok && val != nil {
			mergedNC.Auth = fmt.Sprintf("%v", val)
		}
		if val, ok := rawMap["security"]; ok && val != nil {
			mergedNC.Security = fmt.Sprintf("%v", val)
		}
	} else {
		if c.PostForm("expiryTime") != "" {
			if n, perr := strconv.ParseInt(c.PostForm("expiryTime"), 10, 64); perr == nil {
				mergedNC.ExpiryTime = n
			}
		} else if c.PostForm("expiry_time") != "" {
			if n, perr := strconv.ParseInt(c.PostForm("expiry_time"), 10, 64); perr == nil {
				mergedNC.ExpiryTime = n
			}
		}
		if c.PostForm("totalGB") != "" {
			if n, perr := strconv.ParseInt(c.PostForm("totalGB"), 10, 64); perr == nil {
				mergedNC.TotalGB = n
			}
		} else if c.PostForm("total_gb") != "" {
			if n, perr := strconv.ParseInt(c.PostForm("total_gb"), 10, 64); perr == nil {
				mergedNC.TotalGB = n
			}
		}
		if req.NodeClient.Email != "" {
			mergedNC.Email = req.NodeClient.Email
		}
		if c.PostForm("enable") != "" {
			mergedNC.Enable = c.PostForm("enable") == "true"
		}
		if c.PostForm("reset") != "" {
			if n, perr := strconv.Atoi(c.PostForm("reset")); perr == nil {
				mergedNC.Reset = n
			}
		}
		if req.NodeClient.SubID != "" {
			mergedNC.SubID = req.NodeClient.SubID
		}
		if req.NodeClient.Comment != "" {
			mergedNC.Comment = req.NodeClient.Comment
		}
		if req.NodeClient.Flow != "" {
			mergedNC.Flow = req.NodeClient.Flow
		}
		if req.NodeClient.UUID != "" {
			mergedNC.UUID = req.NodeClient.UUID
		}
		if req.NodeClient.Password != "" {
			mergedNC.Password = req.NodeClient.Password
		}
		if req.NodeClient.SSPassword != "" {
			mergedNC.SSPassword = req.NodeClient.SSPassword
		} else if c.PostForm("ssPassword") != "" {
			mergedNC.SSPassword = c.PostForm("ssPassword")
		} else if c.PostForm("ss_password") != "" {
			mergedNC.SSPassword = c.PostForm("ss_password")
		}
		if req.NodeClient.MasquePass != "" {
			mergedNC.MasquePass = req.NodeClient.MasquePass
		} else if c.PostForm("masquePass") != "" {
			mergedNC.MasquePass = c.PostForm("masquePass")
		} else if c.PostForm("masque_pass") != "" {
			mergedNC.MasquePass = c.PostForm("masque_pass")
		}
	}
	mergedNC.Id = id

	if req.InboundIds == nil {
		formIds := c.PostFormArray("inboundIds")
		if len(formIds) == 0 {
			formIds = c.PostFormArray("inboundIds[]")
		}
		if len(formIds) > 0 {
			var ids []int
			for _, fid := range formIds {
				if n, perr := strconv.Atoi(fid); perr == nil {
					ids = append(ids, n)
				}
			}
			req.InboundIds = &ids
		}
	}

	if user != nil && !user.IsAdmin() && req.InboundIds != nil {
		for _, inId := range *req.InboundIds {
			if !user.CanAccessInbound(inId) {
				pureJsonMsg(c, http.StatusForbidden, false, fmt.Sprintf("Permission denied: inbound %d is outside your allowed scope", inId))
				return
			}
		}
	}

	needRestart, err := a.nodeClientService.Update(&mergedNC)
	if err != nil {
		jsonMsg(c, "Failed to update node client: "+err.Error(), err)
		return
	}

	if req.InboundIds != nil {
		links := make([]service.NodeClientLinkInput, len(*req.InboundIds))
		for i, inId := range *req.InboundIds {
			links[i] = service.NodeClientLinkInput{
				InboundId: inId,
				Flow:      mergedNC.Flow,
			}
		}
		linkRestart, errLink := a.nodeClientService.SetLinks(id, links)
		if errLink != nil {
			logger.Warningf("Failed to update links for client %d: %v", id, errLink)
		}
		if linkRestart {
			needRestart = true
		}
	}

	if needRestart {
		a.xrayService.SetToNeedRestart()
		go func() {
			_ = a.xrayService.RestartXray(true)
		}()
	}
	jsonMsg(c, "Node client updated", nil)
}

func (a *NodeClientController) del(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid node client ID", err)
		return
	}
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		if user.Role == "viewer" || !user.HasPermission("clients:delete") {
			pureJsonMsg(c, http.StatusForbidden, false, "Permission denied to delete client")
			return
		}
	}

	needRestart, err := a.nodeClientService.Delete(id)
	if err != nil {
		jsonMsg(c, "Failed to delete node client", err)
		return
	}

	if needRestart {
		a.xrayService.SetToNeedRestart()
		go func() {
			_ = a.xrayService.RestartXray(true)
		}()
	}
	jsonMsg(c, "Node client deleted", nil)
}

func (a *NodeClientController) bulkDel(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		if user.Role == "viewer" || !user.HasPermission("clients:delete") {
			pureJsonMsg(c, http.StatusForbidden, false, "Permission denied to delete clients")
			return
		}
	}

	var req struct {
		Ids []int `json:"ids" form:"ids"`
	}

	raw, _ := c.GetRawData()
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &req)
	}
	if len(req.Ids) == 0 {
		c.Request.Body = io.NopCloser(bytes.NewBuffer(raw))
		_ = c.ShouldBind(&req)
	}
	if len(req.Ids) == 0 {
		formIds := c.PostFormArray("ids")
		if len(formIds) == 0 {
			formIds = c.PostFormArray("ids[]")
		}
		for _, fid := range formIds {
			if n, perr := strconv.Atoi(fid); perr == nil {
				req.Ids = append(req.Ids, n)
			}
		}
	}

	if len(req.Ids) == 0 {
		jsonMsg(c, "No clients selected", nil)
		return
	}

	needRestart, err := a.nodeClientService.BulkDelete(req.Ids)
	if err != nil {
		jsonMsg(c, "Bulk delete failed: "+err.Error(), err)
		return
	}

	if needRestart {
		a.xrayService.SetToNeedRestart()
		go func() {
			_ = a.xrayService.RestartXray(true)
		}()
	}
	jsonMsg(c, "Node clients deleted", nil)
}

func (a *NodeClientController) getLinks(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid node client ID", err)
		return
	}

	links, err := a.nodeClientService.GetLinks(id)
	if err != nil {
		jsonMsg(c, "Failed to get links", err)
		return
	}
	jsonObj(c, links, nil)
}

func (a *NodeClientController) addLink(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid node client ID", err)
		return
	}

	var req struct {
		InboundId int    `json:"inboundId" form:"inboundId"`
		Flow      string `json:"flow"      form:"flow"`
	}

	err = c.ShouldBind(&req)
	if err != nil {
		jsonMsg(c, "Invalid request", err)
		return
	}

	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		if user.Role == "viewer" || (!user.HasPermission("clients:edit") && !user.HasPermission("clients:update")) || !user.CanAccessInbound(req.InboundId) {
			pureJsonMsg(c, http.StatusForbidden, false, "Permission denied to link to this inbound")
			return
		}
	}

	needRestart, err := a.nodeClientService.AddLink(id, req.InboundId, req.Flow)
	if err != nil {
		jsonMsg(c, "Failed to add link", err)
		return
	}

	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	jsonMsg(c, "Link added", nil)
}

func (a *NodeClientController) removeLink(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid node client ID", err)
		return
	}

	inboundId, err := strconv.Atoi(c.Param("inboundId"))
	if err != nil {
		jsonMsg(c, "Invalid inbound ID", err)
		return
	}

	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		if user.Role == "viewer" || (!user.HasPermission("clients:edit") && !user.HasPermission("clients:update")) || !user.CanAccessInbound(inboundId) {
			pureJsonMsg(c, http.StatusForbidden, false, "Permission denied to remove link from this inbound")
			return
		}
	}

	needRestart, err := a.nodeClientService.RemoveLink(id, inboundId)
	if err != nil {
		jsonMsg(c, "Failed to remove link", err)
		return
	}

	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	jsonMsg(c, "Link removed", nil)
}

func (a *NodeClientController) getTraffic(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid node client ID", err)
		return
	}

	traffic, err := a.nodeClientService.GetAggregatedTraffic(id)
	if err != nil {
		jsonMsg(c, "Failed to get traffic", err)
		return
	}
	jsonObj(c, traffic, nil)
}

func (a *NodeClientController) resetTraffic(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid node client ID", err)
		return
	}
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		if !user.CanResetTraffic || user.Role == "viewer" {
			pureJsonMsg(c, http.StatusForbidden, false, "Permission denied: traffic reset not allowed")
			return
		}
	}

	needsRestart, err := a.nodeClientService.ResetTraffic(id)
	if err != nil {
		jsonMsg(c, "Failed to reset traffic", err)
		return
	}

	if needsRestart {
		a.xrayService.SetToNeedRestart()
		go func() {
			_ = a.xrayService.RestartXray(true)
		}()
	}
	jsonMsg(c, "Traffic reset", nil)
}

func (a *NodeClientController) toggle(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid node client ID", err)
		return
	}
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		if user.Role == "viewer" || (!user.HasPermission("clients:edit") && !user.HasPermission("clients:update")) {
			pureJsonMsg(c, http.StatusForbidden, false, "Permission denied to toggle client status")
			return
		}
	}

	needRestart, err := a.nodeClientService.Toggle(id)
	if err != nil {
		jsonMsg(c, "Failed to toggle node client", err)
		return
	}

	if needRestart {
		a.xrayService.SetToNeedRestart()
		go func() {
			_ = a.xrayService.RestartXray(true)
		}()
	}
	jsonMsg(c, "Node client toggled", nil)
}

func toInt64(val interface{}) int64 {
	switch v := val.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return n
		}
	case string:
		if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			return n
		}
	}
	return 0
}

func toInt(val interface{}) int {
	switch v := val.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case int32:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return int(n)
		}
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return 0
}

func (a *NodeClientController) getDevices(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid client ID", err)
		return
	}
	devices, err := a.nodeClientService.GetDevices(id)
	if err != nil {
		jsonMsg(c, "Failed to get client devices", err)
		return
	}
	jsonObj(c, devices, nil)
}

func (a *NodeClientController) clearDevices(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid client ID", err)
		return
	}
	user := session.GetLoginUser(c)
	if user != nil && !user.IsAdmin() {
		if user.Role == "viewer" {
			pureJsonMsg(c, http.StatusForbidden, false, "Permission denied to clear client devices")
			return
		}
	}
	if err := a.nodeClientService.ClearDevices(id); err != nil {
		jsonMsg(c, "Failed to clear client devices", err)
		return
	}
	jsonMsg(c, "Client devices cleared", nil)
}

