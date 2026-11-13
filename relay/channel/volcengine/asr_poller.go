package volcengine

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ASRPollerConfig 短轮询引擎配置
type ASRPollerConfig struct {
	InitialInterval   time.Duration // 初始轮询间隔，默认 300ms
	MaxInterval       time.Duration // 最大单次间隔，默认 1500ms
	BackoffFactor     float64       // 退避乘子，默认 1.5
	OverallTimeout    time.Duration // 整体超时限制，默认 60s
	MaxNetworkRetries int           // 偶发网络故障最大连续重试次数，默认 3
}

// DefaultASRPollerConfig 获取生产默认轮询配置
func DefaultASRPollerConfig() ASRPollerConfig {
	return ASRPollerConfig{
		InitialInterval:   300 * time.Millisecond,
		MaxInterval:       1500 * time.Millisecond,
		BackoffFactor:     1.5,
		OverallTimeout:    60 * time.Second,
		MaxNetworkRetries: 3,
	}
}

// ASRPoller 自适应退避短轮询调度引擎
type ASRPoller struct {
	config ASRPollerConfig
}

// NewASRPoller 实例化轮询器
func NewASRPoller(config *ASRPollerConfig) *ASRPoller {
	cfg := DefaultASRPollerConfig()
	if config != nil {
		if config.InitialInterval > 0 {
			cfg.InitialInterval = config.InitialInterval
		}
		if config.MaxInterval > 0 {
			cfg.MaxInterval = config.MaxInterval
		}
		if config.BackoffFactor > 1.0 {
			cfg.BackoffFactor = config.BackoffFactor
		}
		if config.OverallTimeout > 0 {
			cfg.OverallTimeout = config.OverallTimeout
		}
		if config.MaxNetworkRetries > 0 {
			cfg.MaxNetworkRetries = config.MaxNetworkRetries
		}
	}
	return &ASRPoller{config: cfg}
}

// Poll 循环查询任务状态，直至终态或超时
func (p *ASRPoller) Poll(ctx context.Context, client *VolcengineASRClient, apiKey string, resourceId string, taskId string) (*VolcengineASRResult, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, p.config.OverallTimeout)
	defer cancel()

	interval := p.config.InitialInterval
	networkRetries := 0

	for {
		// 检查 context 取消状态（客户端是否已主动断开）
		select {
		case <-ctxTimeout.Done():
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("asr polling timed out after %v", p.config.OverallTimeout)
		default:
		}

		queryResp, err := client.QueryTask(ctxTimeout, apiKey, resourceId, taskId)
		if err != nil {
			networkRetries++
			if networkRetries > p.config.MaxNetworkRetries {
				return nil, fmt.Errorf("asr query failed after %d retries: %w", networkRetries, err)
			}
		} else {
			networkRetries = 0
			switch queryResp.StatusCode {
			case StatusSuccess: // 20000000 成功
				return &queryResp.Result, nil
			case StatusProcessing, StatusQueued, "": // 20000001, 20000002 排队或处理中
				// 继续轮询
			default:
				errMsg := queryResp.Message
				if errMsg == "" {
					errMsg = fmt.Sprintf("task failed with status code %s", queryResp.StatusCode)
				}
				return nil, fmt.Errorf("asr task failed [%s]: %s", queryResp.StatusCode, errMsg)
			}
		}

		// 等待退避间隔或响应 Context 取消
		select {
		case <-ctxTimeout.Done():
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("asr polling timed out after %v", p.config.OverallTimeout)
		case <-time.After(interval):
			interval = time.Duration(float64(interval) * p.config.BackoffFactor)
			if interval > p.config.MaxInterval {
				interval = p.config.MaxInterval
			}
		}
	}
}
