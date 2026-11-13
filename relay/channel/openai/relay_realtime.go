package openai

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func OpenaiRealtimeHandler(c *gin.Context, info *relaycommon.RelayInfo) (*types.NewAPIError, *dto.RealtimeUsage) {
	if info == nil || info.ClientWs == nil || info.TargetWs == nil {
		return types.NewError(fmt.Errorf("invalid websocket connection"), types.ErrorCodeBadResponse), nil
	}

	info.IsStream = true
	clientConn := info.ClientWs
	targetConn := info.TargetWs

	clientClosed := make(chan struct{})
	targetClosed := make(chan struct{})
	stop := make(chan struct{})
	var workers sync.WaitGroup
	workers.Add(2)
	var requestDone <-chan struct{}
	if c.Request != nil {
		requestDone = c.Request.Context().Done()
	}
	errChan := make(chan error, 2)

	usage := &dto.RealtimeUsage{}
	localUsage := &dto.RealtimeUsage{}
	sumUsage := &dto.RealtimeUsage{}
	// fork: serialize event accounting and shared relay metadata, never socket I/O.
	var stateMu sync.Mutex
	consumeUsage := func(currentUsage *dto.RealtimeUsage) error {
		return preConsumeUsage(c, info, currentUsage, sumUsage)
	}

	gopool.Go(func() {
		defer close(clientClosed)
		defer workers.Done()
		defer func() {
			if r := recover(); r != nil {
				errChan <- fmt.Errorf("panic in client reader: %v", r)
			}
		}()
		for {
			select {
			case <-stop:
				return
			default:
				_, message, err := clientConn.ReadMessage()
				if err != nil {
					if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
						errChan <- fmt.Errorf("error reading from client: %v", err)
					}
					return
				}

				err = func() error {
					stateMu.Lock()
					defer stateMu.Unlock()
					select {
					case <-stop:
						return nil
					default:
					}
					realtimeEvent := &dto.RealtimeEvent{}
					err = common.Unmarshal(message, realtimeEvent)
					if err != nil {
						return fmt.Errorf("error unmarshalling message: %v", err)
					}

					// fork: an open connection must not admit more generation after
					// another completed request leaves the account in debt. Audio
					// input and session changes can trigger generation without an
					// explicit response.create, so only control-only events bypass.
					switch realtimeEvent.Type {
					case "response.cancel", "input_audio_buffer.clear", "output_audio_buffer.clear",
						"conversation.item.delete", "conversation.item.truncate", "conversation.item.retrieve":
					default:
						apiErr := service.CheckRealtimeGenerationQuota(c, info)
						if apiErr != nil {
							return apiErr
						}
					}

					if realtimeEvent.Type == dto.RealtimeEventTypeSessionUpdate {
						if realtimeEvent.Session != nil {
							if realtimeEvent.Session.Tools != nil {
								info.RealtimeTools = realtimeEvent.Session.Tools
							}
						}
					}

					textToken, audioToken, err := service.CountTokenRealtime(info, *realtimeEvent, info.UpstreamModelName)
					if err != nil {
						return fmt.Errorf("error counting text token: %v", err)
					}
					logger.LogInfo(c, common.LogText("type: %s, textToken: %d, audioToken: %d", realtimeEvent.Type, textToken, audioToken))
					localUsage.TotalTokens += textToken + audioToken
					localUsage.InputTokens += textToken + audioToken
					localUsage.InputTokenDetails.TextTokens += textToken
					localUsage.InputTokenDetails.AudioTokens += audioToken

					return nil
				}()
				if err != nil {
					errChan <- err
					return
				}
				select {
				case <-stop:
					return
				default:
				}
				err = helper.WssString(c, targetConn, string(message))
				if err != nil {
					errChan <- fmt.Errorf("error writing to target: %v", err)
					return
				}
			}
		}
	})

	gopool.Go(func() {
		defer close(targetClosed)
		defer workers.Done()
		defer func() {
			if r := recover(); r != nil {
				errChan <- fmt.Errorf("panic in target reader: %v", r)
			}
		}()
		for {
			select {
			case <-stop:
				return
			default:
				_, message, err := targetConn.ReadMessage()
				if err != nil {
					if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
						errChan <- fmt.Errorf("error reading from target: %v", err)
					}
					return
				}
				err = func() error {
					stateMu.Lock()
					defer stateMu.Unlock()
					select {
					case <-stop:
						return nil
					default:
					}
					info.SetFirstResponseTime()
					realtimeEvent := &dto.RealtimeEvent{}
					err = common.Unmarshal(message, realtimeEvent)
					if err != nil {
						return fmt.Errorf("error unmarshalling message: %v", err)
					}

					if realtimeEvent.Type == dto.RealtimeEventTypeResponseDone {
						realtimeUsage := realtimeEvent.Response.Usage
						if realtimeUsage != nil {
							usage.TotalTokens += realtimeUsage.TotalTokens
							usage.InputTokens += realtimeUsage.InputTokens
							usage.OutputTokens += realtimeUsage.OutputTokens
							usage.InputTokenDetails.AudioTokens += realtimeUsage.InputTokenDetails.AudioTokens
							usage.InputTokenDetails.CachedTokens += realtimeUsage.InputTokenDetails.CachedTokens
							usage.InputTokenDetails.TextTokens += realtimeUsage.InputTokenDetails.TextTokens
							usage.OutputTokenDetails.AudioTokens += realtimeUsage.OutputTokenDetails.AudioTokens
							usage.OutputTokenDetails.TextTokens += realtimeUsage.OutputTokenDetails.TextTokens
							err := consumeUsage(usage)
							// consumeUsage already owns this response's accumulated usage,
							// including on settlement failure; the tail must not add it again.
							usage = &dto.RealtimeUsage{}
							localUsage = &dto.RealtimeUsage{}
							if err != nil {
								return types.NewError(err, types.ErrorCodeUpdateDataError, types.ErrOptionWithSkipRetry())
							}
						} else {
							textToken, audioToken, err := service.CountTokenRealtime(info, *realtimeEvent, info.UpstreamModelName)
							if err != nil {
								return fmt.Errorf("error counting text token: %v", err)
							}
							logger.LogInfo(c, common.LogText("type: %s, textToken: %d, audioToken: %d", realtimeEvent.Type, textToken, audioToken))
							localUsage.TotalTokens += textToken + audioToken
							info.IsFirstRequest = false
							localUsage.InputTokens += textToken + audioToken
							localUsage.InputTokenDetails.TextTokens += textToken
							localUsage.InputTokenDetails.AudioTokens += audioToken
							err = consumeUsage(localUsage)
							localUsage = &dto.RealtimeUsage{}
							if err != nil {
								return types.NewError(err, types.ErrorCodeUpdateDataError, types.ErrOptionWithSkipRetry())
							}
						}
						logger.LogInfo(c, common.LogText("realtime streaming sumUsage: %v", sumUsage))
						logger.LogInfo(c, common.LogText("realtime streaming localUsage: %v", localUsage))
						logger.LogInfo(c, common.LogText("realtime streaming localUsage: %v", localUsage))

					} else if realtimeEvent.Type == dto.RealtimeEventTypeSessionUpdated || realtimeEvent.Type == dto.RealtimeEventTypeSessionCreated {
						realtimeSession := realtimeEvent.Session
						if realtimeSession != nil {
							// update audio format
							info.InputAudioFormat = common.GetStringIfEmpty(realtimeSession.InputAudioFormat, info.InputAudioFormat)
							info.OutputAudioFormat = common.GetStringIfEmpty(realtimeSession.OutputAudioFormat, info.OutputAudioFormat)
						}
					} else {
						textToken, audioToken, err := service.CountTokenRealtime(info, *realtimeEvent, info.UpstreamModelName)
						if err != nil {
							return fmt.Errorf("error counting text token: %v", err)
						}
						logger.LogInfo(c, common.LogText("type: %s, textToken: %d, audioToken: %d", realtimeEvent.Type, textToken, audioToken))
						localUsage.TotalTokens += textToken + audioToken
						localUsage.OutputTokens += textToken + audioToken
						localUsage.OutputTokenDetails.TextTokens += textToken
						localUsage.OutputTokenDetails.AudioTokens += audioToken
					}

					return nil
				}()
				if err != nil {
					errChan <- err
					return
				}
				select {
				case <-stop:
					return
				default:
				}
				err = helper.WssString(c, clientConn, string(message))
				if err != nil {
					errChan <- fmt.Errorf("error writing to client: %v", err)
					return
				}
			}
		}
	})

	var newAPIError *types.NewAPIError
	select {
	case <-clientClosed:
	case <-targetClosed:
	case err := <-errChan:
		//return service.OpenAIErrorWrapper(err, "realtime_error", http.StatusInternalServerError), nil
		logger.LogError(c, common.LogText("realtime error: %s", err.Error()))
		// Preserve the native debt error for the websocket error frame while
		// still returning observed usage for final settlement by the caller.
		errors.As(err, &newAPIError)
	case <-requestDone:
	}

	// fork: the caller owns the final client error frame. Quiesce both data
	// writers before returning, while preserving a healthy client write side.
	close(stop)
	_ = targetConn.Close()
	_ = clientConn.UnderlyingConn().SetReadDeadline(time.Now())
	joined := make(chan struct{})
	go func() { workers.Wait(); close(joined) }()
	timer := time.NewTimer(time.Second)
	select {
	case <-joined:
	case <-timer.C:
		// A peer that will not read may leave a data write blocked indefinitely.
		_ = clientConn.Close()
		<-joined
	}
	timer.Stop()
	// Worker completion can race its queued error notification. Preserve the
	// native error even when completion won the first select.
	for len(errChan) > 0 {
		err := <-errChan
		if newAPIError == nil {
			errors.As(err, &newAPIError)
		}
	}

	if usage.TotalTokens != 0 {
		_ = consumeUsage(usage)
	}

	if localUsage.TotalTokens != 0 {
		_ = consumeUsage(localUsage)
	}

	// check usage total tokens, if 0, use local usage

	return newAPIError, sumUsage
}

func preConsumeUsage(ctx *gin.Context, info *relaycommon.RelayInfo, usage *dto.RealtimeUsage, totalUsage *dto.RealtimeUsage) error {
	if usage == nil || totalUsage == nil {
		return fmt.Errorf("invalid usage pointer")
	}

	totalUsage.TotalTokens += usage.TotalTokens
	totalUsage.InputTokens += usage.InputTokens
	totalUsage.OutputTokens += usage.OutputTokens
	totalUsage.InputTokenDetails.CachedTokens += usage.InputTokenDetails.CachedTokens
	totalUsage.InputTokenDetails.TextTokens += usage.InputTokenDetails.TextTokens
	totalUsage.InputTokenDetails.AudioTokens += usage.InputTokenDetails.AudioTokens
	totalUsage.OutputTokenDetails.TextTokens += usage.OutputTokenDetails.TextTokens
	totalUsage.OutputTokenDetails.AudioTokens += usage.OutputTokenDetails.AudioTokens
	// clear usage
	err := service.PreWssConsumeQuota(ctx, info, usage)
	return err
}
