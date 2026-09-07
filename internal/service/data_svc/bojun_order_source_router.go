package data_svc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"gin-biz-web-api/pkg/config"
)

type BojunOrderSourceMode string

const (
	BojunOrderSourceAPI    BojunOrderSourceMode = "api"
	BojunOrderSourceOracle BojunOrderSourceMode = "oracle"
)

var (
	ErrBojunOrderSourceModeInvalid = errors.New("bojun order source mode is invalid")
	ErrBojunOracleSyncDisabled     = errors.New("bojun Oracle sync is disabled")
)

type bojunAPIOrderSyncer interface {
	SyncRecentOrders(context.Context) (*BojunOrderSyncResult, error)
	PreviewOrders(context.Context, string, string) (*BojunOrderSyncResult, error)
	SyncOrders(context.Context, string, string) (*BojunOrderSyncResult, error)
}

type bojunOracleOrderSyncer interface {
	SyncIncremental(context.Context) (*BojunOrderSyncResult, error)
	PreviewByStatusTime(context.Context, string, string) (*BojunOrderSyncResult, error)
	SyncByStatusTime(context.Context, string, string) (*BojunOrderSyncResult, error)
}

type BojunOrderSourceRouter struct {
	mode          BojunOrderSourceMode
	modeErr       error
	oracleEnabled bool
	api           bojunAPIOrderSyncer
	oracle        bojunOracleOrderSyncer
}

func NewBojunOrderSourceRouter() *BojunOrderSourceRouter {
	return newBojunOrderSourceRouter(configuredBojunOrderSourceMode())
}

func NewBojunOrderSourceRouterForMode(mode string) *BojunOrderSourceRouter {
	return newBojunOrderSourceRouter(mode)
}

func newBojunOrderSourceRouter(rawMode string) *BojunOrderSourceRouter {
	mode, err := NormalizeBojunOrderSourceMode(rawMode)
	return &BojunOrderSourceRouter{
		mode: mode, modeErr: err, oracleEnabled: bojunOracleSyncEnabled(),
		api: NewBojunOrderService(), oracle: NewBojunOracleOrderService(),
	}
}

func NormalizeBojunOrderSourceMode(value string) (BojunOrderSourceMode, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", string(BojunOrderSourceAPI):
		return BojunOrderSourceAPI, nil
	case string(BojunOrderSourceOracle):
		return BojunOrderSourceOracle, nil
	default:
		return "", fmt.Errorf("%w: must be api or oracle", ErrBojunOrderSourceModeInvalid)
	}
}

func (router *BojunOrderSourceRouter) SourceMode() string {
	if router == nil || router.mode == "" {
		return string(BojunOrderSourceAPI)
	}
	return string(router.mode)
}

func (router *BojunOrderSourceRouter) SyncRecentOrders(ctx context.Context) (*BojunOrderSyncResult, error) {
	if err := router.validate(); err != nil {
		return nil, err
	}
	var result *BojunOrderSyncResult
	var err error
	if router.mode == BojunOrderSourceOracle {
		result, err = router.oracle.SyncIncremental(ctx)
	} else {
		result, err = router.api.SyncRecentOrders(ctx)
	}
	return withBojunSourceMode(result, router.mode), err
}

func (router *BojunOrderSourceRouter) PreviewOrders(ctx context.Context, startTime, endTime string) (*BojunOrderSyncResult, error) {
	if err := router.validate(); err != nil {
		return nil, err
	}
	var result *BojunOrderSyncResult
	var err error
	if router.mode == BojunOrderSourceOracle {
		result, err = router.oracle.PreviewByStatusTime(ctx, startTime, endTime)
	} else {
		result, err = router.api.PreviewOrders(ctx, startTime, endTime)
	}
	return withBojunSourceMode(result, router.mode), err
}

func (router *BojunOrderSourceRouter) SyncOrders(ctx context.Context, startTime, endTime string) (*BojunOrderSyncResult, error) {
	if err := router.validate(); err != nil {
		return nil, err
	}
	var result *BojunOrderSyncResult
	var err error
	if router.mode == BojunOrderSourceOracle {
		result, err = router.oracle.SyncByStatusTime(ctx, startTime, endTime)
	} else {
		result, err = router.api.SyncOrders(ctx, startTime, endTime)
	}
	return withBojunSourceMode(result, router.mode), err
}

func (router *BojunOrderSourceRouter) validate() error {
	if router == nil {
		return fmt.Errorf("bojun order source router is unavailable")
	}
	if router.modeErr != nil {
		return router.modeErr
	}
	if router.mode == BojunOrderSourceOracle {
		if !router.oracleEnabled {
			return ErrBojunOracleSyncDisabled
		}
		if router.oracle == nil {
			return fmt.Errorf("bojun Oracle order service is unavailable")
		}
		return nil
	}
	if router.api == nil {
		return fmt.Errorf("bojun API order service is unavailable")
	}
	return nil
}

func withBojunSourceMode(result *BojunOrderSyncResult, mode BojunOrderSourceMode) *BojunOrderSyncResult {
	if result != nil {
		result.SourceMode = string(mode)
	}
	return result
}

func configuredBojunOrderSourceMode() string {
	return bojunEnvString(
		"BOJUN_ORDER_SOURCE_MODE",
		config.GetString("Bojun.OrderSourceMode", string(BojunOrderSourceAPI)),
	)
}

func bojunOracleSyncEnabled() bool {
	if value := strings.ToLower(strings.TrimSpace(os.Getenv("BOJUN_ORACLE_SYNC_ENABLED"))); value != "" {
		return value == "1" || value == "true" || value == "yes"
	}
	return config.GetBool("Bojun.OracleSyncEnabled", false)
}
