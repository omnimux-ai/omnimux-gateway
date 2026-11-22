package controller

import (
	"github.com/QuantumNous/new-api/pkg/analytics"
	"github.com/gin-gonic/gin"
)

var trackSignupEvent = analytics.Track

// signupAnalytics captures only the allowed metadata for the post-commit callback.
func signupAnalytics(c *gin.Context, method string) func(int) {
	data := analytics.SignupData(method, c.GetHeader("X-OmniMux-Device-Id"),
		c.GetHeader("X-OmniMux-App-Version"), c.GetHeader("Referer"))
	return func(userID int) {
		trackSignupEvent(analytics.Event{Name: analytics.EventSignUpSuccess, UserID: userID, Data: data})
	}
}
