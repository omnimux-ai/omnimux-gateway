package common

import (
	"crypto/subtle"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type verificationValue struct {
	code     string
	time     time.Time
	attempts int
}

const (
	EmailVerificationPurpose = "v"
	PasswordResetPurpose     = "r"
	maxVerificationAttempts  = 5
)

var verificationMutex sync.Mutex
var verificationMap map[string]verificationValue
var verificationMapMaxSize = 10
var VerificationValidMinutes = 10

func GenerateVerificationCode(length int) string {
	code := uuid.New().String()
	code = strings.Replace(code, "-", "", -1)
	if length == 0 {
		return code
	}
	return code[:length]
}

func RegisterVerificationCodeWithKey(key string, code string, purpose string) {
	verificationMutex.Lock()
	defer verificationMutex.Unlock()
	verificationMap[purpose+key] = verificationValue{
		code: code,
		time: time.Now(),
	}
	if len(verificationMap) > verificationMapMaxSize {
		removeExpiredPairs()
	}
}

func verifyCodeLocked(mapKey string, code string, consume bool) bool {
	value, okay := verificationMap[mapKey]
	if !okay {
		return false
	}
	if int(time.Since(value.time).Seconds()) >= VerificationValidMinutes*60 {
		delete(verificationMap, mapKey)
		return false
	}
	if code == "" || subtle.ConstantTimeCompare([]byte(code), []byte(value.code)) != 1 {
		value.attempts++
		if value.attempts >= maxVerificationAttempts {
			delete(verificationMap, mapKey)
		} else {
			verificationMap[mapKey] = value
		}
		return false
	}
	if consume {
		delete(verificationMap, mapKey)
	}
	return true
}

func VerifyCodeWithKey(key string, code string, purpose string) bool {
	verificationMutex.Lock()
	defer verificationMutex.Unlock()
	return verifyCodeLocked(purpose+key, code, false)
}

// ConsumeCodeWithKey verifies a verification code in constant time and
// atomically removes it so concurrent requests cannot replay the same code.
func ConsumeCodeWithKey(key string, code string, purpose string) bool {
	verificationMutex.Lock()
	defer verificationMutex.Unlock()
	return verifyCodeLocked(purpose+key, code, true)
}

func DeleteKey(key string, purpose string) {
	verificationMutex.Lock()
	defer verificationMutex.Unlock()
	delete(verificationMap, purpose+key)
}

// no lock inside, so the caller must lock the verificationMap before calling!
func removeExpiredPairs() {
	now := time.Now()
	for key := range verificationMap {
		if int(now.Sub(verificationMap[key].time).Seconds()) >= VerificationValidMinutes*60 {
			delete(verificationMap, key)
		}
	}
}

func init() {
	verificationMutex.Lock()
	defer verificationMutex.Unlock()
	verificationMap = make(map[string]verificationValue)
}
