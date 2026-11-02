package social

// fork: social-provider-isolation — explicit source is a constraint, never an account rewrite.

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

var (
	ErrProviderInvalid         = errors.New("provider must be tiktok_direct or zernio")
	ErrAccountProviderMismatch = errors.New("account-provider-mismatch")
	ErrPostProviderMismatch    = errors.New("post-provider-mismatch")
)

func validateProvider(provider string) error {
	if provider != ProviderTikTokDirect && provider != ProviderZernio {
		return ErrProviderInvalid
	}
	return nil
}

func validateAccountProvider(provider string, accounts []*model.SocialAccount) error {
	for _, account := range accounts {
		if account.Provider != provider || (provider == ProviderTikTokDirect && account.Platform != "tiktok") {
			return ErrAccountProviderMismatch
		}
	}
	return nil
}

// validateIdempotentPost binds a replay to the same provider and account set.
func validateIdempotentPost(post *model.SocialPost, in CreatePostInput) error {
	if post.Provider != in.Provider {
		return ErrPostProviderMismatch
	}
	var snapshot struct {
		AccountIDs []int64 `json:"account_ids"`
	}
	if err := common.UnmarshalJsonStr(post.RequestSnapshot, &snapshot); err != nil || len(snapshot.AccountIDs) == 0 {
		return fmt.Errorf("%w: stored account set is unavailable", ErrPostProviderMismatch)
	}
	expected := make(map[int64]bool, len(in.AccountIDs))
	for _, id := range in.AccountIDs {
		expected[id] = true
	}
	actual := make(map[int64]bool, len(snapshot.AccountIDs))
	for _, id := range snapshot.AccountIDs {
		actual[id] = true
		if !expected[id] {
			return fmt.Errorf("%w: idempotency account set differs", ErrPostProviderMismatch)
		}
	}
	if len(actual) != len(expected) {
		return fmt.Errorf("%w: idempotency account set differs", ErrPostProviderMismatch)
	}
	return nil
}
