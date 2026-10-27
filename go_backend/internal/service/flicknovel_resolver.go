package service

import (
	"go_backend/internal/service/client/flicknovel"

	"go.uber.org/zap"
)

// OrderResolveContext 订单充值类型解析上下文
type OrderResolveContext struct {
	Dto *flicknovel.OrderDto
}

// FlicknovelOrderTypeResolver 番茄司南订单充值类型解析组件
// 严格依据更新后的 OpenAPI right_type 枚举判定：
// 1-买赠充值, 2-订阅会员, 3-整本购买, 4-一次性会员, 5-金币包
type FlicknovelOrderTypeResolver struct {
	logger *zap.Logger
}

func NewFlicknovelOrderTypeResolver(logger *zap.Logger) *FlicknovelOrderTypeResolver {
	return &FlicknovelOrderTypeResolver{logger: logger}
}

// Resolve 核心判定方法 (0=非订阅单次购买, 1=订阅会员)
func (r *FlicknovelOrderTypeResolver) Resolve(ctx *OrderResolveContext) int {
	if ctx == nil || ctx.Dto == nil {
		return 0
	}
	if ctx.Dto.IsSubscription() {
		return 1
	}
	return 0
}
