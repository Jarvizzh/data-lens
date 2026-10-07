package service

import (
	"encoding/json"
	"strings"
	"time"

	"go_backend/internal/service/client/flicknovel"

	"go.uber.org/zap"
)

// FallbackSubsStartDateUTC 兜底为订阅策略的起始日期 (UTC 时间 2026-09-20 开始)
var FallbackSubsStartDateUTC = time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)

// TemplatePriceDetail 充值模板高级详情结构
type TemplatePriceDetail struct {
	PriceMap               map[int]int
	AmbiguousPrices        map[int]bool
	HasIntroOffer          bool
	FirstPriceMap          map[int]int
	NoFirstPriceMap        map[int]int
	FirstAmbiguousPrices   map[int]bool
	NoFirstAmbiguousPrices map[int]bool
}

// OrderResolveContext 订单充值类型解析上下文
type OrderResolveContext struct {
	Dto                    *flicknovel.OrderDto
	PromotionID            string
	OrderAmountCent        int
	RenewType              int // 1=首单, 2=老用户复充
	PayTimeBJ              time.Time
	PayTimeUTC             time.Time
	HasSubscribed          bool      // 用户历史是否有过订阅记录
	LatestSubsPayTime      time.Time // 用户最近一次订阅支付时间 (北京时间)
	TemplatePriceMap       map[int]int
	FirstPriceMap          map[int]int
	NoFirstPriceMap        map[int]int
	AmbiguousPrices        map[int]bool
	FirstAmbiguousPrices   map[int]bool
	NoFirstAmbiguousPrices map[int]bool
	TemplateHasIntroOffer  bool
}

// FlicknovelOrderTypeResolver 番茄司南订单充值类型解析策略组件
type FlicknovelOrderTypeResolver struct {
	logger *zap.Logger
}

func NewFlicknovelOrderTypeResolver(logger *zap.Logger) *FlicknovelOrderTypeResolver {
	return &FlicknovelOrderTypeResolver{logger: logger}
}

// Resolve 核心判定方法 (0=单充/代币, 1=时长订阅)
func (r *FlicknovelOrderTypeResolver) Resolve(ctx *OrderResolveContext) int {
	if ctx == nil {
		return 0
	}

	// =========================================================================
	// 【第一优先级: OpenAPI 显式字段检查 (Zero-Coupling)】
	// 若番茄官方未来在接口返回 benefit_type 或 product_id，直接生效！
	// =========================================================================
	if ctx.Dto != nil {
		if ctx.Dto.BenefitType > 0 {
			if ctx.Dto.BenefitType == 2 {
				return 1
			}
			return 0
		}
		if pID := strings.TrimSpace(ctx.Dto.ProductID); pID != "" {
			pLower := strings.ToLower(pID)
			if strings.Contains(pLower, "sub") || strings.Contains(pLower, "vip") ||
				strings.Contains(pLower, "week") || strings.Contains(pLower, "month") ||
				strings.Contains(pLower, "year") || strings.Contains(pLower, "day") {
				return 1
			}
			if strings.Contains(pLower, "coin") || strings.Contains(pLower, "token") ||
				strings.Contains(pLower, "recharge") {
				return 0
			}
		}
	}

	amountCent := ctx.OrderAmountCent
	renewType := ctx.RenewType // 1=首单, 2=老用户复充

	// =========================================================================
	// 【第二优先级: 首充专属字典 vs 非首充专属字典独立路由】
	// =========================================================================
	if renewType == 1 {
		// --- 场景 A: 首充订单 (renew_type == 1) ---
		if ctx.FirstAmbiguousPrices != nil && ctx.FirstAmbiguousPrices[amountCent] {
			return r.resolveAmbiguousPrice(ctx)
		}
		if ctx.FirstPriceMap != nil {
			if val, ok := ctx.FirstPriceMap[amountCent]; ok {
				return val
			}
		}
	} else if renewType == 2 {
		// --- 场景 B: 非首充/复充订单 (renew_type == 2) ---
		if ctx.NoFirstAmbiguousPrices != nil && ctx.NoFirstAmbiguousPrices[amountCent] {
			return r.resolveAmbiguousPrice(ctx)
		}
		if ctx.NoFirstPriceMap != nil {
			if val, ok := ctx.NoFirstPriceMap[amountCent]; ok {
				return val
			}
		}
	}

	// =========================================================================
	// 【第三优先级: 跨池优雅降级与全局消歧】
	// 若在指定池未配置，降级尝试全局合并池消歧或直接映射
	// =========================================================================
	if ctx.AmbiguousPrices != nil && ctx.AmbiguousPrices[amountCent] {
		return r.resolveAmbiguousPrice(ctx)
	}

	if ctx.TemplatePriceMap != nil {
		if val, ok := ctx.TemplatePriceMap[amountCent]; ok {
			return val
		}
	}

	// =========================================================================
	// 【第四优先级: 模板未匹配档位兜底策略】
	// 从 2026-09-20 (UTC) 开始，若模板价格字典找不到档位，兜底策略为订阅 (1)；
	// 2026-09-20 (UTC) 之前仍默认兜底为代币单充 (0)。
	// =========================================================================
	payDateUTC := ctx.PayTimeUTC
	if payDateUTC.IsZero() && !ctx.PayTimeBJ.IsZero() {
		payDateUTC = ctx.PayTimeBJ.UTC()
	}
	if !payDateUTC.IsZero() && !payDateUTC.Before(FallbackSubsStartDateUTC) {
		return 1
	}

	return 0
}

// resolveAmbiguousPrice 针对单充与订阅同金额（如 39.99 / 3999 美分）的时序生命周期消歧
func (r *FlicknovelOrderTypeResolver) resolveAmbiguousPrice(ctx *OrderResolveContext) int {
	renewType := ctx.RenewType
	amountCent := ctx.OrderAmountCent

	// 1. 首单逻辑 (renew_type == 1):
	// 如果模板中配有低价首购优惠 (例如 $19.99 或 $29.99 的 Intro Price)，
	// 用户首次充值如果付的是 $39.99，说明根本没选置顶的优惠订阅，而是主动选择了 $39.99 代币单充档位！
	if renewType == 1 {
		if ctx.TemplateHasIntroOffer {
			return 0
		}
		// 若模板完全没有首充优惠（纯原价模板），检查是否在非首充池中明确为代币 (0)
		if ctx.NoFirstPriceMap != nil {
			if val, ok := ctx.NoFirstPriceMap[amountCent]; ok && val == 0 {
				return 0
			}
		}
		// 根据置顶订阅偏好优先判定为订阅
		return 1
	}

	// 2. 复充逻辑 (renew_type == 2):
	// 检查该用户此前是否有过订阅记录
	if ctx.HasSubscribed && !ctx.LatestSubsPayTime.IsZero() && !ctx.PayTimeBJ.IsZero() {
		daysDiff := int(ctx.PayTimeBJ.Sub(ctx.LatestSubsPayTime).Hours() / 24)
		// 周订续费窗口：距离上一次订阅订单大约 7 天 (允许 5 ~ 9 天网络或系统宽限期，或者隔周 12 ~ 16 天)
		if (daysDiff >= 5 && daysDiff <= 9) || (daysDiff >= 12 && daysDiff <= 16) {
			return 1
		}
		// 月订续费窗口：约 30 天 (27 ~ 33 天)
		if daysDiff >= 27 && daysDiff <= 33 {
			return 1
		}
	}

	// 用户此前无订阅记录，或复充时间完全不符合订阅续订周期律 -> 判定为代币单充
	return 0
}

// ParseTemplatePriceDetail 解析单个充值模板 JSON (对齐 Java parsePriceTypeDetail)
func ParseTemplatePriceDetail(rawPayload string) *TemplatePriceDetail {
	detailObj := &TemplatePriceDetail{
		PriceMap:               make(map[int]int),
		AmbiguousPrices:        make(map[int]bool),
		FirstPriceMap:          make(map[int]int),
		NoFirstPriceMap:        make(map[int]int),
		FirstAmbiguousPrices:   make(map[int]bool),
		NoFirstAmbiguousPrices: make(map[int]bool),
	}

	if strings.TrimSpace(rawPayload) == "" {
		return detailObj
	}

	var root map[string]interface{}
	if err := json.Unmarshal([]byte(rawPayload), &root); err != nil {
		return detailObj
	}

	detail, ok := root["detail"].(map[string]interface{})
	if !ok || detail == nil {
		return detailObj
	}

	allCoinPrices := make(map[int]bool)
	allSubsPrices := make(map[int]bool)
	firstCoinPrices := make(map[int]bool)
	firstSubsPrices := make(map[int]bool)
	noFirstCoinPrices := make(map[int]bool)
	noFirstSubsPrices := make(map[int]bool)

	for _, platformVal := range detail {
		platformNode, ok := platformVal.(map[string]interface{})
		if !ok || platformNode == nil {
			continue
		}

		var firstProductLists [][]interface{}
		var noFirstProductLists [][]interface{}

		addSliceIfPresent := func(target *[][]interface{}, arrVal interface{}) {
			if s, ok := arrVal.([]interface{}); ok && len(s) > 0 {
				*target = append(*target, s)
			}
		}

		addSliceIfPresent(&firstProductLists, platformNode["first_top_products"])
		addSliceIfPresent(&firstProductLists, platformNode["first_products"])
		addSliceIfPresent(&noFirstProductLists, platformNode["nofirst_top_products"])
		addSliceIfPresent(&noFirstProductLists, platformNode["nofirst_products"])

		if rechargeNode, ok := platformNode["recharge"].(map[string]interface{}); ok && rechargeNode != nil {
			addSliceIfPresent(&firstProductLists, rechargeNode["first_products"])
			addSliceIfPresent(&noFirstProductLists, rechargeNode["nofirst_products"])
		}
		if subscribeNode, ok := platformNode["subscribe"].(map[string]interface{}); ok && subscribeNode != nil {
			addSliceIfPresent(&firstProductLists, subscribeNode["first_products"])
			addSliceIfPresent(&noFirstProductLists, subscribeNode["nofirst_products"])
		}

		// 1. 处理首充商品池 (first_products)
		for _, list := range firstProductLists {
			for _, itemVal := range list {
				item, ok := itemVal.(map[string]interface{})
				if !ok {
					continue
				}
				product, _ := item["product"].(map[string]interface{})
				benefitType := getIntField(product, "benefit_type", getIntField(item, "benefit_type", 0))
				priceCents := getIntField(product, "price_cents", getIntField(item, "price_cents", 0))
				discountPriceCents := getIntField(product, "discount_price_cents", getIntField(item, "discount_price_cents", 0))
				customPriceCents := getIntField(item, "custom_price_cents", 0)

				if benefitType == 2 {
					// 订阅产品
					if discountPriceCents > 0 {
						firstSubsPrices[discountPriceCents] = true
						detailObj.FirstPriceMap[discountPriceCents] = 1
						allSubsPrices[discountPriceCents] = true
						detailObj.PriceMap[discountPriceCents] = 1
						if priceCents > discountPriceCents {
							detailObj.HasIntroOffer = true
						}
					}
					if customPriceCents > 0 {
						firstSubsPrices[customPriceCents] = true
						detailObj.FirstPriceMap[customPriceCents] = 1
						allSubsPrices[customPriceCents] = true
						detailObj.PriceMap[customPriceCents] = 1
					}
					if priceCents > 0 {
						firstSubsPrices[priceCents] = true
						if _, exists := detailObj.FirstPriceMap[priceCents]; !exists {
							detailObj.FirstPriceMap[priceCents] = 1
						}
						allSubsPrices[priceCents] = true
						if _, exists := detailObj.PriceMap[priceCents]; !exists {
							detailObj.PriceMap[priceCents] = 1
						}
					}
				} else if benefitType == 1 {
					// 代币单充
					if discountPriceCents > 0 {
						firstCoinPrices[discountPriceCents] = true
						if _, exists := detailObj.FirstPriceMap[discountPriceCents]; !exists {
							detailObj.FirstPriceMap[discountPriceCents] = 0
						}
						allCoinPrices[discountPriceCents] = true
						if _, exists := detailObj.PriceMap[discountPriceCents]; !exists {
							detailObj.PriceMap[discountPriceCents] = 0
						}
					}
					if customPriceCents > 0 {
						firstCoinPrices[customPriceCents] = true
						if _, exists := detailObj.FirstPriceMap[customPriceCents]; !exists {
							detailObj.FirstPriceMap[customPriceCents] = 0
						}
						allCoinPrices[customPriceCents] = true
						if _, exists := detailObj.PriceMap[customPriceCents]; !exists {
							detailObj.PriceMap[customPriceCents] = 0
						}
					}
					if priceCents > 0 {
						firstCoinPrices[priceCents] = true
						if _, exists := detailObj.FirstPriceMap[priceCents]; !exists {
							detailObj.FirstPriceMap[priceCents] = 0
						}
						allCoinPrices[priceCents] = true
						if _, exists := detailObj.PriceMap[priceCents]; !exists {
							detailObj.PriceMap[priceCents] = 0
						}
					}
				}
			}
		}

		// 2. 处理非首充商品池 (nofirst_products)
		for _, list := range noFirstProductLists {
			for _, itemVal := range list {
				item, ok := itemVal.(map[string]interface{})
				if !ok {
					continue
				}
				product, _ := item["product"].(map[string]interface{})
				benefitType := getIntField(product, "benefit_type", getIntField(item, "benefit_type", 0))
				priceCents := getIntField(product, "price_cents", getIntField(item, "price_cents", 0))
				discountPriceCents := getIntField(product, "discount_price_cents", getIntField(item, "discount_price_cents", 0))
				customPriceCents := getIntField(item, "custom_price_cents", 0)

				if benefitType == 2 {
					// 订阅产品
					if discountPriceCents > 0 {
						noFirstSubsPrices[discountPriceCents] = true
						detailObj.NoFirstPriceMap[discountPriceCents] = 1
						allSubsPrices[discountPriceCents] = true
						detailObj.PriceMap[discountPriceCents] = 1
					}
					if customPriceCents > 0 {
						noFirstSubsPrices[customPriceCents] = true
						detailObj.NoFirstPriceMap[customPriceCents] = 1
						allSubsPrices[customPriceCents] = true
						detailObj.PriceMap[customPriceCents] = 1
					}
					if priceCents > 0 {
						noFirstSubsPrices[priceCents] = true
						if _, exists := detailObj.NoFirstPriceMap[priceCents]; !exists {
							detailObj.NoFirstPriceMap[priceCents] = 1
						}
						allSubsPrices[priceCents] = true
						if _, exists := detailObj.PriceMap[priceCents]; !exists {
							detailObj.PriceMap[priceCents] = 1
						}
					}
				} else if benefitType == 1 {
					// 代币单充
					if discountPriceCents > 0 {
						noFirstCoinPrices[discountPriceCents] = true
						if _, exists := detailObj.NoFirstPriceMap[discountPriceCents]; !exists {
							detailObj.NoFirstPriceMap[discountPriceCents] = 0
						}
						allCoinPrices[discountPriceCents] = true
						if _, exists := detailObj.PriceMap[discountPriceCents]; !exists {
							detailObj.PriceMap[discountPriceCents] = 0
						}
					}
					if customPriceCents > 0 {
						noFirstCoinPrices[customPriceCents] = true
						if _, exists := detailObj.NoFirstPriceMap[customPriceCents]; !exists {
							detailObj.NoFirstPriceMap[customPriceCents] = 0
						}
						allCoinPrices[customPriceCents] = true
						if _, exists := detailObj.PriceMap[customPriceCents]; !exists {
							detailObj.PriceMap[customPriceCents] = 0
						}
					}
					if priceCents > 0 {
						noFirstCoinPrices[priceCents] = true
						if _, exists := detailObj.NoFirstPriceMap[priceCents]; !exists {
							detailObj.NoFirstPriceMap[priceCents] = 0
						}
						allCoinPrices[priceCents] = true
						if _, exists := detailObj.PriceMap[priceCents]; !exists {
							detailObj.PriceMap[priceCents] = 0
						}
					}
				}
			}
		}
	}

	// 计算价格冲突集合
	for p := range allCoinPrices {
		if allSubsPrices[p] {
			detailObj.AmbiguousPrices[p] = true
		}
	}
	for p := range firstCoinPrices {
		if firstSubsPrices[p] {
			detailObj.FirstAmbiguousPrices[p] = true
		}
	}
	for p := range noFirstCoinPrices {
		if noFirstSubsPrices[p] {
			detailObj.NoFirstAmbiguousPrices[p] = true
		}
	}

	return detailObj
}

func getIntField(m map[string]interface{}, key string, def int) int {
	if m == nil {
		return def
	}
	val, ok := m[key]
	if !ok || val == nil {
		return def
	}
	switch v := val.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return int(i)
		}
	}
	return def
}
