package flicknovel

type BaseResponse[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

// 推广链接
type PromotionQueryRequest struct {
	DistAppID int64 `json:"dist_app_id,omitempty"`
	PageIndex int   `json:"page_index"`
	PageSize  int   `json:"page_size"`
}

type PromotionQueryData struct {
	TotalCount int64           `json:"total_count"`
	Promotions []PromotionItem `json:"promotions"`
}

type PromotionItem struct {
	PromotionID     string `json:"promotion_id"`
	PromotionName   string `json:"promotion_name"`
	RechargeTplID   string `json:"recharge_tpl_id"`
	RechargeTplName string `json:"recharge_tpl_name"`
	DistAppID       int64  `json:"dist_app_id"`
	DramaID         string `json:"drama_id"`
	DramaTitle      string `json:"drama_title"`
	ChapterID       string `json:"chapter_id"`
	ChapterTitle    string `json:"chapter_title"`
	MediaChannel    int    `json:"media_channel"`
}

// 充值模板
type RechargeTemplateQueryRequest struct {
	TemplateIDs []string `json:"template_ids,omitempty"`
	DistAppID   int64    `json:"dist_app_id,omitempty"`
}

type RechargeTemplateData struct {
	Templates []RechargeTemplateItem `json:"templates"`
}

type RechargeTemplateItem struct {
	TemplateID      string `json:"template_id"`
	Name            string `json:"name"`
	DistAppID       int64  `json:"dist_app_id"`
	PriceConfigJSON string `json:"price_config_json"`
}

// 订单
type OrderQueryRequest struct {
	DistAppID int64  `json:"dist_app_id,omitempty"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	PageIndex int    `json:"page_index"`
	PageSize  int    `json:"page_size"`
}

type OrderQueryData struct {
	TotalCount int64       `json:"total_count"`
	Orders     []OrderItem `json:"orders"`
}

type OrderItem struct {
	OrderID         string `json:"order_id"`
	PromotionID     string `json:"promotion_id"`
	DeviceID        string `json:"device_id"`
	UserID          string `json:"user_id"`
	OrderAmountCent int    `json:"order_amount_cent"` // 分
	OrderAmountUSD  string `json:"order_amount_usd"`
	IsSubs          int    `json:"is_subs"`
	RenewType       int    `json:"renew_type"`
	PayState        int    `json:"pay_state"`
	PayTime         string `json:"pay_time"`      // yyyy-MM-dd HH:mm:ss
	RegisterTime    string `json:"register_time"`  // yyyy-MM-dd HH:mm:ss
	RefundStatus    int    `json:"refund_status"`
}

// 染色归因
type RelationQueryRequest struct {
	DistAppID int64  `json:"dist_app_id,omitempty"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	PageIndex int    `json:"page_index"`
	PageSize  int    `json:"page_size"`
}

type RelationQueryData struct {
	TotalCount int64          `json:"total_count"`
	Relations  []RelationItem `json:"relations"`
}

type RelationItem struct {
	RelationID             string `json:"relation_id"`
	DeviceID               string `json:"device_id"`
	PromotionID            string `json:"promotion_id"`
	PromotionCode          string `json:"promotion_code"`
	AdID                   string `json:"ad_id"`
	AdsetID                string `json:"adset_id"`
	CampaignID             string `json:"campaign_id"`
	AdAccountID            string `json:"ad_account_id"`
	RelationBeginTime      string `json:"relation_begin_time"` // 北京时间
	RelationBeginTimestamp int64  `json:"relation_begin_timestamp"`
	MediaChannel           string `json:"media_channel"`
	Platform               string `json:"platform"`
	AppID                  string `json:"app_id"`
}
