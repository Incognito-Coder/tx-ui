package controller

import (
	"net/http"
	"strconv"

	"x-ui/internal/database/model"
	"x-ui/internal/web/service"

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
	g.POST("/update/:id", a.update)
	g.POST("/del/:id", a.del)
	g.POST("/bulkDel", a.bulkDel)
	g.GET("/:id/links", a.getLinks)
	g.POST("/:id/addLink", a.addLink)
	g.POST("/:id/removeLink/:inboundId", a.removeLink)
	g.GET("/:id/traffic", a.getTraffic)
	g.POST("/:id/resetTraffic", a.resetTraffic)
	g.POST("/:id/toggle", a.toggle)
}

func (a *NodeClientController) list(c *gin.Context) {
	clients, err := a.nodeClientService.GetAll()
	if err != nil {
		jsonMsg(c, "Failed to get node clients", err)
		return
	}
	jsonObj(c, clients, nil)
}

func (a *NodeClientController) getOne(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid node client ID", err)
		return
	}

	nc, err := a.nodeClientService.GetByID(id)
	if err != nil {
		jsonMsg(c, "Node client not found", err)
		c.Status(http.StatusNotFound)
		return
	}
	jsonObj(c, nc, nil)
}

func (a *NodeClientController) create(c *gin.Context) {
	nc := &model.NodeClient{}
	err := c.ShouldBind(nc)
	if err != nil {
		jsonMsg(c, "Failed to create node client", err)
		return
	}

	err = a.nodeClientService.Create(nc)
	if err != nil {
		jsonMsg(c, "Failed to create node client", err)
		return
	}

	jsonObj(c, nc, nil)
}

func (a *NodeClientController) update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid node client ID", err)
		return
	}

	nc := &model.NodeClient{Id: id}
	err = c.ShouldBind(nc)
	if err != nil {
		jsonMsg(c, "Failed to update node client", err)
		return
	}

	err = a.nodeClientService.Update(nc)
	if err != nil {
		jsonMsg(c, "Failed to update node client", err)
		return
	}

	a.xrayService.SetToNeedRestart()
	jsonMsg(c, "Node client updated", nil)
}

func (a *NodeClientController) del(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid node client ID", err)
		return
	}

	err = a.nodeClientService.Delete(id)
	if err != nil {
		jsonMsg(c, "Failed to delete node client", err)
		return
	}

	a.xrayService.SetToNeedRestart()
	jsonMsg(c, "Node client deleted", nil)
}

func (a *NodeClientController) bulkDel(c *gin.Context) {
	var req struct {
		Ids []int `json:"ids" form:"ids"`
	}

	err := c.ShouldBind(&req)
	if err != nil {
		jsonMsg(c, "Invalid request", err)
		return
	}

	if len(req.Ids) == 0 {
		jsonMsg(c, "Bulk delete failed", err)
		return
	}

	err = a.nodeClientService.BulkDelete(req.Ids)
	if err != nil {
		jsonMsg(c, "Bulk delete failed", err)
		return
	}

	a.xrayService.SetToNeedRestart()
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

	err = a.nodeClientService.AddLink(id, req.InboundId, req.Flow)
	if err != nil {
		jsonMsg(c, "Failed to add link", err)
		return
	}

	a.xrayService.SetToNeedRestart()
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

	err = a.nodeClientService.RemoveLink(id, inboundId)
	if err != nil {
		jsonMsg(c, "Failed to remove link", err)
		return
	}

	a.xrayService.SetToNeedRestart()
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

	needsRestart, err := a.nodeClientService.ResetTraffic(id)
	if err != nil {
		jsonMsg(c, "Failed to reset traffic", err)
		return
	}

	if needsRestart {
		a.xrayService.SetToNeedRestart()
	}
	jsonMsg(c, "Traffic reset", nil)
}

func (a *NodeClientController) toggle(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid node client ID", err)
		return
	}

	nc, err := a.nodeClientService.GetByID(id)
	if err != nil {
		jsonMsg(c, "Node client not found", err)
		c.Status(http.StatusNotFound)
		return
	}

	nc.Enable = !nc.Enable

	err = a.nodeClientService.Update(nc)
	if err != nil {
		jsonMsg(c, "Failed to toggle node client", err)
		return
	}

	a.xrayService.SetToNeedRestart()
	jsonMsg(c, "Node client toggled", nil)
}
