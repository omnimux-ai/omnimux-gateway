package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/modelcontracts"
)

// fork: OmniMux — guards the additive contract field on the public catalog row.

func TestPricingRowPublishesTheContractWhenPresent(t *testing.T) {
	contract, ok := modelcontracts.For("seedance-2-5")
	if !ok {
		t.Skip("seedance-2-5 has no shipped contract")
	}

	body, err := common.Marshal(Pricing{ModelName: "seedance-2-5", Contract: &contract})
	if err != nil {
		t.Fatalf("marshal pricing row: %v", err)
	}
	serialized := string(body)

	if !strings.Contains(serialized, `"contract"`) {
		t.Fatalf("row carries no contract field: %s", serialized)
	}
	if !strings.Contains(serialized, `"model_id":"seedance-2-5"`) {
		t.Errorf("contract is missing its model id: %s", serialized)
	}
	if !strings.Contains(serialized, `"durations_seconds"`) {
		t.Errorf("contract is missing the declared duration envelope: %s", serialized)
	}
}

func TestPricingRowWithoutAContractIsUnchanged(t *testing.T) {
	body, err := common.Marshal(Pricing{ModelName: "no-such-model"})
	if err != nil {
		t.Fatalf("marshal pricing row: %v", err)
	}
	if strings.Contains(string(body), "contract") {
		t.Errorf("a row without a contract must omit the field entirely: %s", string(body))
	}
}
