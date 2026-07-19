package xray

type ClientTraffic struct {
	Id           int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	InboundId    int    `json:"inboundId" form:"inboundId" gorm:"uniqueIndex:idx_email_inbound"`
	Enable       bool   `json:"enable" form:"enable"`
	Email        string `json:"email" form:"email" gorm:"uniqueIndex:idx_email_inbound"`
	Up           int64  `json:"up" form:"up"`
	Down         int64  `json:"down" form:"down"`
	ExpiryTime   int64  `json:"expiryTime" form:"expiryTime"`
	Total        int64  `json:"total" form:"total"`
	Reset        int    `json:"reset" form:"reset" gorm:"default:0"`
	NodeClientId *int   `json:"nodeClientId" gorm:"index"` // NULL for regular clients; non-NULL links to node_clients.id
}
