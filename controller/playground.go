package controller

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
)

// setupPlaygroundUserContext writes the signed-in user's cache into the request
// context so downstream relay code sees acceptUnsetRatio and the user's group.
func setupPlaygroundUserContext(c *gin.Context) error {
	userId := c.GetInt("id")
	userCache, err := model.GetUserCache(userId)
	if err != nil {
		return err
	}
	userCache.WriteContext(c)
	return nil
}

// setupPlaygroundTokenContext bridges a session-authenticated request to the
// token-shaped context the relay layer expects, by minting an in-memory token
// bound to the signed-in user and their resolved group. Nothing is persisted:
// the token exists only for this request.
//
// The caller renders the returned error; a nil result means the relay may run.
func setupPlaygroundTokenContext(c *gin.Context, format types.RelayFormat, name string) *types.NewAPIError {
	if c.GetBool("use_access_token") {
		return types.NewError(errors.New(i18n.T(c, i18n.MsgRelayAccessTokenUnsupported)), types.ErrorCodeAccessDenied, types.ErrOptionWithSkipRetry())
	}

	relayInfo, err := relaycommon.GenRelayInfo(c, format, nil, nil)
	if err != nil {
		return types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	if err := setupPlaygroundUserContext(c); err != nil {
		return types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
	}

	userId := c.GetInt("id")
	tempToken := &model.Token{
		UserId: userId,
		Name:   fmt.Sprintf("%s-%s", name, relayInfo.UsingGroup),
		Group:  relayInfo.UsingGroup,
	}
	_ = middleware.SetupContextForToken(c, tempToken)
	return nil
}

func Playground(c *gin.Context) {
	var newAPIError *types.NewAPIError

	defer func() {
		if newAPIError != nil {
			c.JSON(newAPIError.StatusCode, gin.H{
				"error": newAPIError.ToOpenAIError(),
			})
		}
	}()

	newAPIError = setupPlaygroundTokenContext(c, types.RelayFormatOpenAI, "playground")
	if newAPIError != nil {
		return
	}

	Relay(c, types.RelayFormatOpenAI)
}

// PlaygroundTask submits a generation task on behalf of the signed-in user, so
// the browser playground works with the session cookie alone. It reuses the
// same task-plugin chain, channel distribution and native precharge/settle
// path as /v1/video/generations; only the credential differs.
func PlaygroundTask(c *gin.Context) {
	newAPIError := setupPlaygroundTokenContext(c, types.RelayFormatTask, "playground-task")
	if newAPIError != nil {
		c.JSON(newAPIError.StatusCode, gin.H{
			"error": newAPIError.ToOpenAIError(),
		})
		return
	}

	RelayTaskPluginEndpoint(c, RelayTask)
}
