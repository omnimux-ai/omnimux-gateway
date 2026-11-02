package dto

import "testing"

// 渠道级「按请求参数选上游模型」：一个公开 ID 可以按请求参数（例如图片的 size）
// 落到不同的上游 SKU。只决定上游名，不参与计费（计费继续按公开 ID）。
func TestResolveModelByParam(t *testing.T) {
	cfg := &AdvancedCustomConfig{
		ModelByParam: map[string]ParamModelRouting{
			"gpt-image-2.5": {
				Param: "size",
				Rules: []ParamModelRule{
					{AnyOf: []string{"2048x2048", "3840x2160"}, Model: "image2.5-pro"},
				},
			},
		},
	}
	lookup := func(params map[string]string) func(string) string {
		return func(param string) string { return params[param] }
	}

	cases := []struct {
		name    string
		model   string
		params  map[string]string
		want    string
		wantHit bool
	}{
		{"命中规则", "gpt-image-2.5", map[string]string{"size": "2048x2048"}, "image2.5-pro", true},
		{"命中且大小写与空白无关", "gpt-image-2.5", map[string]string{"size": " 3840X2160 "}, "image2.5-pro", true},
		{"未命中规则 → 不改写", "gpt-image-2.5", map[string]string{"size": "1024x1024"}, "", false},
		{"未配置的模型 → 不改写", "nano-banana-pro", map[string]string{"size": "2048x2048"}, "", false},
		{"参数为空 → 不改写", "gpt-image-2.5", map[string]string{"size": ""}, "", false},
		{"请求里没有该参数 → 不改写", "gpt-image-2.5", map[string]string{}, "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := cfg.ResolveModelByParam(tc.model, lookup(tc.params))
			if got != tc.want || ok != tc.wantHit {
				t.Fatalf("ResolveModelByParam(%q) = (%q, %v), want (%q, %v)", tc.model, got, ok, tc.want, tc.wantHit)
			}
		})
	}
}

func TestResolveModelByParamInactiveShapes(t *testing.T) {
	lookup := func(string) string { return "2048x2048" }

	var nilCfg *AdvancedCustomConfig
	if _, ok := nilCfg.ResolveModelByParam("gpt-image-2.5", lookup); ok {
		t.Fatal("nil 配置必须不生效")
	}

	empty := &AdvancedCustomConfig{}
	if _, ok := empty.ResolveModelByParam("gpt-image-2.5", lookup); ok {
		t.Fatal("未配置 model_by_param 时必须完全不生效")
	}

	noRule := &AdvancedCustomConfig{
		ModelByParam: map[string]ParamModelRouting{"gpt-image-2.5": {Rules: nil}},
	}
	if _, ok := noRule.ResolveModelByParam("gpt-image-2.5", lookup); ok {
		t.Fatal("规则为空时必须不生效")
	}

	blankModel := &AdvancedCustomConfig{
		ModelByParam: map[string]ParamModelRouting{
			"gpt-image-2.5": {Rules: []ParamModelRule{{AnyOf: []string{"2048x2048"}, Model: "  "}}},
		},
	}
	if _, ok := blankModel.ResolveModelByParam("gpt-image-2.5", lookup); ok {
		t.Fatal("规则里的上游名为空时必须不生效，避免把请求打到一个空模型名")
	}

	if _, ok := noRule.ResolveModelByParam("gpt-image-2.5", nil); ok {
		t.Fatal("lookup 为 nil 时必须不生效，不得 panic")
	}
}
