package controller

import (
	"github.com/QuantumNous/new-api/pkg/analytics"
	"github.com/gin-gonic/gin"
)

var trackSignupEvent = analytics.Track

// trackCommittedSignup is called only by self-service flows after the new account commits.
func trackCommittedSignup(c *gin.Context, userID int, method string) {
	referrer := c.GetHeader("Referer")
	if len(referrer) > 2048 {
		referrer = ""
	}
	trackSignupEvent(analytics.Event{
		Name:   analytics.EventSignUpSuccess,
		UserID: userID,
		Data: map[string]any{
			"method":   method,
			"referrer": referrer, // The collector strips private URL components before sending.
		},
	})
}
