package types

import "testing"

func TestMidjourneyUpstreamModel(t *testing.T) {
	cases := []struct {
		model     string
		operation string
		wantModel string
		wantRatio float64
		wantKnown bool
	}{
		{"mj-v7", "", "mj-v7", 1, true},
		{"mj-v7", "generate", "mj-v7", 1, true},
		{"mj-v7", "upscale", "mj-v7-upscale", 1.5, true},
		{"mj-v7", "variation", "mj-v7-variation", 1.5, true},
		{"mj-v7", "enhance", "mj-v7-enhance", 1, true},
		{"mj-v7", "pan", "mj-v7-pan", 1, true},
		{"mj-v7", "outpaint", "mj-v7-outpaint", 1, true},
		{"mj-v7", "remove-bg", "mj-v7-remove-bg", 1, true},
		{"mj-v7", "inpaint", "mj-v7-inpaint", 1.5, true},
		{"mj-v8-1", "", "mj-v8.1", 1, true},
		{"mj-v8-1", "remove-bg", "mj-v8.1-remove-bg", 1, true},
		{"mj-v8-1", "upscale", "mj-v8.1-upscale", 1.5, true},
		{"mj-v8-1", "variation", "mj-v8.1-variation", 1.5, true},
		{"mj-v8-1", "edit", "mj-v8.1-edit", 1.5, true},
		{"mj-v8-1", "upload-paint", "mj-v8.1-upload-paint", 1.5, true},
		// Unknown operation on a known version.
		{"mj-v7", "nope", "", 0, false},
		// Legacy operation-suffixed ids are not part of the registry.
		{"mj-v7-upscale", "upscale", "", 0, false},
		{"midjourney", "", "", 0, false},
		{"gpt-image-2", "upscale", "", 0, false},
	}
	for _, tc := range cases {
		model, ratio, ok := MidjourneyUpstreamModel(tc.model, tc.operation)
		if ok != tc.wantKnown {
			t.Fatalf("%s/%s: known=%v want %v", tc.model, tc.operation, ok, tc.wantKnown)
		}
		if !ok {
			continue
		}
		if model != tc.wantModel {
			t.Errorf("%s/%s: upstream=%q want %q", tc.model, tc.operation, model, tc.wantModel)
		}
		if ratio != tc.wantRatio {
			t.Errorf("%s/%s: ratio=%v want %v", tc.model, tc.operation, ratio, tc.wantRatio)
		}
	}
}

// Both versions share one ladder now: the operation sits on its measured upstream
// tier (CNY 0.54 = 1x, CNY 0.81 = 1.5x) and both versions anchor on the same base.
// The 2026-08-31 VIP snapshot (CNY 0.27 for v7 generation) no longer matches what
// upstream bills, which is why the anchor moved.
func TestMidjourneyVersionPriceParity(t *testing.T) {
	const price = 0.113446 // both versions: CNY 0.54 / 6.8 / 0.7
	want := map[string]float64{
		"generate":     0.113446,
		"enhance":      0.113446,
		"pan":          0.113446,
		"outpaint":     0.113446,
		"remove-bg":    0.113446,
		"upscale":      0.170169,
		"variation":    0.170169,
		"edit":         0.170169,
		"inpaint":      0.170169,
		"remix":        0.170169,
		"retexture":    0.170169,
		"upload-paint": 0.170169,
	}
	for _, operation := range []string{"generate", "enhance", "pan", "outpaint", "remove-bg", "upscale", "variation", "edit", "inpaint", "remix", "retexture", "upload-paint"} {
		_, ratioV7, okV7 := MidjourneyUpstreamModel("mj-v7", operation)
		_, ratioV8, okV8 := MidjourneyUpstreamModel("mj-v8-1", operation)
		if !okV7 || !okV8 {
			t.Fatalf("%s: expected both versions to support the operation (v7=%v v8=%v)", operation, okV7, okV8)
		}
		if ratioV7 != ratioV8 {
			t.Errorf("%s: ratio differs between versions (v7=%v v8.1=%v)", operation, ratioV7, ratioV8)
		}
		price := price * ratioV7
		if diff := price - want[operation]; diff > 1e-5 || diff < -1e-5 {
			t.Errorf("%s: price=$%.6f want $%.6f", operation, price, want[operation])
		}
	}
}

// The measured parameter tiers: turbo = 2x, creative upscale = 4/3x, and the
// documented hd = 1.5x / draft = 1x.
func TestMidjourneyMeasuredParameterTiers(t *testing.T) {
	const price = 0.113446
	cases := []struct {
		name      string
		operation string
		quality   string
		speed     string
		typeValue string
		want      float64
	}{
		{"generation", "", "", "", "", 1},
		{"hd generation", "", "hd", "", "", 1.5},
		{"draft generation", "", "", "draft", "", 1},
		{"turbo pan (measured 2x)", "pan", "", "turbo", "", 2},
		{"standard upscale", "upscale", "", "", "standard", 1.5},
		{"creative upscale (measured 4/3x)", "upscale", "", "", "creative", 1.5 * 4.0 / 3.0},
		{"hd creative upscale stacks", "upscale", "hd", "", "creative", 1.5 * 1.5 * 4.0 / 3.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			multiplier, ok := MidjourneyMultiplier("mj-v8-1", tc.operation, tc.quality, tc.speed, tc.typeValue)
			if !ok {
				t.Fatal("expected a priced combination")
			}
			if diff := multiplier - tc.want; diff > 1e-9 || diff < -1e-9 {
				t.Errorf("multiplier=%v want %v", multiplier, tc.want)
			}
			t.Logf("%s -> $%.6f", tc.name, price*multiplier)
		})
	}
}

// An unknown `type` on a priced operation would change upstream's price without
// changing ours, so it must be refused.
func TestMidjourneyUnpricedTypeIsRefused(t *testing.T) {
	if reason := MidjourneyParameterError("mj-v8-1", "upscale", "", "", "mega"); reason == "" {
		t.Error("unknown upscale type must be refused")
	}
	if reason := MidjourneyParameterError("mj-v8-1", "upscale", "", "", "creative"); reason != "" {
		t.Errorf("creative upscale is measured and must be accepted, got %q", reason)
	}
	if reason := MidjourneyParameterError("mj-v7", "pan", "", "turbo", ""); reason != "" {
		t.Errorf("turbo is measured and must be accepted, got %q", reason)
	}
}

// Every public id must exist on both versions, so a client can ask for the same
// action on either model.
func TestMidjourneyOperationSetsAreSubsetOfEachOther(t *testing.T) {
	v7 := MidjourneyOperationMultipliers["mj-v7"]
	v81 := MidjourneyOperationMultipliers["mj-v8-1"]
	for operation := range v81 {
		if _, ok := v7[operation]; !ok {
			t.Errorf("v7 lacks operation %q that v8.1 supports", operation)
		}
	}
	if len(v7) != len(v81) {
		t.Errorf("operation counts differ: v7=%d v8.1=%d", len(v7), len(v81))
	}
}

func TestMidjourneyOperationFromBody(t *testing.T) {
	body := []byte(`{"model":"mj-v7","prompt":"a cat","model_params":{"operation":"upscale","task_id":"t1"}}`)
	operation, ok := MidjourneyOperationFromBody(body)
	if !ok || operation != "upscale" {
		t.Fatalf("operation=%q ok=%v", operation, ok)
	}
	if ratio := MidjourneyOperationRatioFromBody("mj-v7", body); ratio != 1.5 {
		t.Errorf("ratio=%v want 1.5", ratio)
	}
	if ratio := MidjourneyOperationRatioFromBody("gpt-image-2", body); ratio != 1 {
		t.Errorf("non-MJ model ratio=%v want 1", ratio)
	}
	if _, ok := MidjourneyOperationFromBody([]byte(`{"prompt":"x"}`)); ok {
		t.Error("body without model_params must not report an operation")
	}
	if ratio := MidjourneyOperationRatioFromBody("mj-v7", []byte(`{"model_params":{"operation":"bogus"}}`)); ratio != 1 {
		t.Errorf("unknown operation ratio=%v want 1", ratio)
	}
}

func TestMidjourneyOperationFromRawParams(t *testing.T) {
	operation, ok := MidjourneyOperationFromRawParams([]byte(`{"operation":"EDit","task_id":"t"}`))
	if !ok || operation != "edit" {
		t.Fatalf("operation=%q ok=%v", operation, ok)
	}
	if _, ok := MidjourneyOperationFromRawParams([]byte(`{"task_id":"t"}`)); ok {
		t.Error("missing operation must not be reported")
	}
	if _, ok := MidjourneyOperationFromRawParams(nil); ok {
		t.Error("nil raw params must not be reported")
	}
}
