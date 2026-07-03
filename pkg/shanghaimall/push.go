package shanghaimall

import (
	"context"
	"fmt"
)

func Push(ctx context.Context, target Target, order RetailOrder) (*PushResult, error) {
	switch target {
	case TargetJialiCheng:
		return PushJialiCheng(ctx, order)
	case TargetPanlong:
		return PushPanlong(ctx, order)
	case TargetQiantan:
		return PushQiantan(ctx, order)
	case TargetShangsheng:
		return PushShangsheng(ctx, order)
	case TargetXintiandi:
		return PushXintiandi(ctx, order)
	default:
		return nil, fmt.Errorf("unsupported shanghai mall target %q", target)
	}
}
