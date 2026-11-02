package relay

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	tasksora "github.com/QuantumNous/new-api/relay/channel/task/sora"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTaskPathBillingExpressionContract verifies that all models reachable by
// the task submit path (async image list ∪ -task suffix) which use tiered_expr
// satisfy the task path pricing invariant: any fixed pricing must be strictly
// request-scoped (no usage meters).
func TestTaskPathBillingExpressionContract(t *testing.T) {
	baselinePath := filepath.Join("..", "scripts", "ops", "config-baseline", "options.json")
	data, err := os.ReadFile(baselinePath)
	require.NoError(t, err, "reading baseline options.json")

	var options struct {
		BillingMode map[string]string `json:"billing_setting.billing_mode"`
		BillingExpr map[string]string `json:"billing_setting.billing_expr"`
	}
	require.NoError(t, common.Unmarshal(data, &options))

	taskModelsChecked := 0
	for model, mode := range options.BillingMode {
		if mode != "tiered_expr" {
			continue
		}
		expr := options.BillingExpr[model]
		require.NotEmpty(t, expr, "model %s marked tiered_expr must have expression", model)

		isTaskReachable := tasksora.IsAsyncImageModel(model) || strings.HasSuffix(model, "-task")
		if !isTaskReachable {
			continue
		}
		taskModelsChecked++

		t.Run(model, func(t *testing.T) {
			_, err := billingexpr.CompileFromCache(expr)
			require.NoError(t, err, "compiling expression for %s", model)

			if billingexpr.UsesFixedPricing(expr) {
				assert.True(t, billingexpr.IsRequestScopedFixedPricing(expr),
					"task-reachable model %s with fixed pricing must be request-scoped (no usage meters), got: %s",
					model, expr)
			}
		})
	}

	assert.GreaterOrEqual(t, taskModelsChecked, 4, "must cover at least gpt-image-2.5 and three -task models")
}
