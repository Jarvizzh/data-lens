package flicknovel

type BaseResponse[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

// 推广链接
type PromotionQueryRequest struct {
	Email            string  `json:"email,omitempty"`
	PromotionID      string  `json:"promotion_id,omitempty"`
	CreatedStartTime int64   `json:"created_start_time,omitempty"`
	CreatedEndTime   int64   `json:"created_end_time,omitempty"`
	DistAppID        []int64 `json:"dist_app_id,omitempty"`
	Genres           []int64 `json:"genres,omitempty"`
	Page             int64   `json:"page"`
	PageSize         int64   `json:"page_size"`
	PageIndex        int     `json:"page_index,omitempty"`
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
	DisAppID  int64  `json:"dis_app_id,omitempty"`
	DistAppID int64  `json:"dist_app_id,omitempty"`
	Email     string `json:"email,omitempty"`
	Page      int64  `json:"page"`
	PageSize  int64  `json:"page_size"`
}

type RechargeTemplateData struct {
	Templates []RechargeTemplateItem `json:"recharge_templates"`
}

type RechargeTemplateItem struct {
	TemplateID      string `json:"template_id"`
	Name            string `json:"name"`
	DistAppID       int64  `json:"dist_app_id"`
	PriceConfigJSON string `json:"price_config_json"`
}

// 订单
type OrderQueryRequest struct {
	BeginTs  int64 `json:"begin_ts"`
	EndTs    int64 `json:"end_ts"`
	Page     int64 `json:"page"`
	PageSize int64 `json:"page_size"`
}

type OrderQueryData struct {
	TotalCount int64       `json:"total_count"`
	Orders     []OrderItem `json:"orders"`
}

type OrderItem struct {
	OrderID         string `json:"order_id"`
	DeviceID        string `json:"device_id"`
	AdID            string `json:"ad_id"`
	AdsetID         string `json:"adset_id"`
	CampaignID      string `json:"campaign_id"`
	AdAccountID     string `json:"ad_account_id"`
	PromotionID     string `json:"promotion_id"`
	PromotionCode   string `json:"promotion_code"`
	DistributorID   string `json:"distributor_id"`
	ContentID       string `json:"content_id"`
	Language        string `json:"language"`
	MediaChannel    string `json:"media_channel"`
	CreatedAt       string `json:"created_at"`   // 秒级时间戳字符串
	CompletedAt     string `json:"completed_at"` // 秒级时间戳字符串
	USPrice         string `json:"us_price"`     // 美元金额，如 "39.99"
	RelationID      string `json:"relation_id"`
	AppID           string `json:"app_id"`
	AppName         string `json:"app_name"`
	BenefitType     int    `json:"benefit_type"`
	RechargeType    int    `json:"recharge_type"`
	ProductID       string `json:"product_id"`

	// 兼容旧字段
	UserID          string `json:"user_id,omitempty"`
	OrderAmountCent int    `json:"order_amount_cent,omitempty"`
	OrderAmountUSD  string `json:"order_amount_usd,omitempty"`
	IsSubs          int    `json:"is_subs,omitempty"`
	RenewType       int    `json:"renew_type,omitempty"`
	PayState        int    `json:"pay_state,omitempty"`
	PayTime         string `json:"pay_time,omitempty"`
	RegisterTime    string `json:"register_time,omitempty"`
	RefundStatus    int    `json:"refund_status,omitempty"`
}

// 染色归因
type RelationQueryRequest struct {
	BeginTs  int64 `json:"begin_ts"`
	EndTs    int64 `json:"end_ts"`
	Page     int64 `json:"page"`
	PageSize int64 `json:"page_size"`
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
	RelationBeginTime      string `json:"relation_begin_time"` // 秒级时间戳字符串
	RelationBeginTimestamp int64  `json:"relation_begin_timestamp"`
	MediaChannel           string `json:"media_channel"`
	Platform               string `json:"platform"`
	AppID                  string `json:"app_id"`
}

// 类型别名对齐
type OrderDto = OrderItem
type PromotionDto = PromotionItem
type RelationDto = RelationItem
type RechargeTemplateDto = RechargeTemplateItem
