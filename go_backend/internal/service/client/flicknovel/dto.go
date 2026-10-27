package flicknovel

import (
	"strconv"
	"strings"
)

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
	OrderID             string `json:"order_id"`
	DeviceID            string `json:"device_id"`
	AdID                string `json:"ad_id"`
	AdsetID             string `json:"adset_id"`
	CampaignID          string `json:"campaign_id"`
	AdAccountID         string `json:"ad_account_id"`
	PromotionID         string `json:"promotion_id"`
	PromotionCode       string `json:"promotion_code"`
	DistributorID       string `json:"distributor_id"`
	DistributorEmail    string `json:"distributor_email"`
	DistributorName     string `json:"distributor_name"`
	ContentID           string `json:"content_id"`
	ContentName         string `json:"content_name"`
	Language            string `json:"language"`
	MediaChannel        string `json:"media_channel"`
	CreatedAt           string `json:"created_at"`            // 秒级时间戳字符串
	CompletedAt         string `json:"completed_at"`          // 秒级时间戳字符串
	USPrice             string `json:"us_price"`              // 实际支付美元金额，如 "0.99" / "19.99"
	CurrentProductPrice string `json:"current_product_price"` // 商品现价(原价)，如 "19.99"
	RelationID          string `json:"relation_id"`
	AppID               string `json:"app_id"`
	AppName             string `json:"app_name"`
	ProductID           string `json:"product_id"`
	RightType           string `json:"right_type"`    // 商品权益类型: 1-买赠充值, 2-订阅会员, 3-整本购买, 4-一次性会员, 5-金币包
	Period              string `json:"period"`        // 周期单位: 1-周, 2-月, 3-年, 4-日
	PeriodCount         string `json:"period_count"`  // 周期数量: 如 "1", "3"
	RechargeType        string `json:"recharge_type"` // 订阅类型: 1-首订, 2-续订, 100-单充
	RefundStatus        string `json:"refund_status"` // 退款状态: 0-未退款, 4-已退款
	RefundTime          string `json:"refund_time"`   // 退款时间(秒级时间戳字符串)
}

// GetRightTypeInt 解析商品权益类型枚举 (1-买赠充值, 2-订阅会员, 3-整本购买, 4-一次性会员, 5-金币包)
func (o *OrderItem) GetRightTypeInt() int {
	v, _ := strconv.Atoi(strings.TrimSpace(o.RightType))
	return v
}

// IsSubscription 判定是否为订阅会员订单 (仅 right_type == 2 为订阅会员)
func (o *OrderItem) IsSubscription() bool {
	return o.GetRightTypeInt() == 2
}

// GetSubPeriodDays 结合 period(1-周, 2-月, 3-年, 4-日) 与 period_count 计算订阅周期天数
func (o *OrderItem) GetSubPeriodDays() int {
	pUnit, _ := strconv.Atoi(strings.TrimSpace(o.Period))
	pCount, _ := strconv.Atoi(strings.TrimSpace(o.PeriodCount))
	if pCount <= 0 {
		pCount = 1
	}
	switch pUnit {
	case 1: // 周
		return 7 * pCount
	case 2: // 月 (如 period=2, period_count=3 为季卡 90 天)
		return 30 * pCount
	case 3: // 年
		return 365 * pCount
	case 4: // 日
		return 1 * pCount
	default:
		return 1
	}
}

// GetRefundStatusInt 严格解析司南接口返回的 refund_status (0-未退款, 4-已退款)
func (o *OrderItem) GetRefundStatusInt() int {
	v, _ := strconv.Atoi(strings.TrimSpace(o.RefundStatus))
	return v
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
