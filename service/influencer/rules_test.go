package influencer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTaxonomyCounts(t *testing.T) {
	vis := map[string]int{"normal": 0, "freak": 0, "total": 0}
	humanCategories, humanOptions := 0, 0
	for _, c := range Taxonomy().Categories {
		categoryVisible := false
		for _, o := range c.Options {
			optionVisible := false
			for _, tier := range o.VisibleIn {
				if _, human := vis[tier]; human {
					vis[tier]++
					optionVisible = true
				}
			}
			if optionVisible {
				humanOptions++
				categoryVisible = true
			}
		}
		if categoryVisible {
			humanCategories++
		}
	}
	assert.Equal(t, 18, humanCategories)
	assert.Equal(t, 186, humanOptions)
	assert.Equal(t, map[string]int{"normal": 140, "freak": 183, "total": 186}, vis)
	assert.NotEmpty(t, ETag())
}

func TestNormalizeRejectsUnknown(t *testing.T) {
	if _, err := NormalizeSelection("normal", Selection{"nope": {"x"}}); err == nil {
		t.Fatal("expected unknown category error")
	}
	if _, err := NormalizeSelection("normal", Selection{"gender": {"nope"}}); err == nil {
		t.Fatal("expected unknown option error")
	}
	if _, err := NormalizeSelection("notatier", Selection{"gender": {"female"}}); err == nil {
		t.Fatal("expected invalid tier error")
	}
}

func TestNormalizeDropsInvisible(t *testing.T) {
	// pr_egg and head_tiny are not visible in normal tier per dataset
	norm, err := NormalizeSelection("normal", Selection{
		"proportions": {"pr_egg", "pr_shoulders"},
		"freak_head":  {"head_tiny"},
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if contains(norm["proportions"], "pr_egg") || len(norm["freak_head"]) != 0 {
		t.Fatalf("invisible option kept: %v", norm)
	}
	if !contains(norm["proportions"], "pr_shoulders") {
		t.Fatalf("visible option dropped: %v", norm)
	}
}

func TestAccNoneExclusive(t *testing.T) {
	norm, err := NormalizeSelection("total", Selection{
		"accessory": {"acc_glasses", "acc_none"},
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got := norm["accessory"]; len(got) != 1 || got[0] != "acc_none" {
		t.Fatalf("acc_none did not clear others: %v", got)
	}
}

func TestSlotExclusivityLastWins(t *testing.T) {
	norm, err := NormalizeSelection("total", Selection{
		"freak_face": {"fn_fulllips", "ff_lips_3", "fn_freckles"},
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	got := norm["freak_face"]
	if contains(got, "fn_fulllips") {
		t.Fatalf("same-slot option not replaced: %v", got)
	}
	if !contains(got, "ff_lips_3") || !contains(got, "fn_freckles") {
		t.Fatalf("unexpected kept set: %v", got)
	}
}

func TestMaxCapKeepsFirst(t *testing.T) {
	// fn_eyebags(eyes) fn_mole(marks) fn_cheekbones(cheeks) fn_fulllips(lips)
	// are 4 distinct slots; fn_widemouth(lips) both replaces lips and is the
	// 5th pick, exceeding max=4
	norm, err := NormalizeSelection("total", Selection{
		"freak_face": {"fn_eyebags", "fn_mole", "fn_cheekbones", "fn_fulllips", "fn_widemouth"},
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	got := norm["freak_face"]
	if len(got) != 4 {
		t.Fatalf("max cap broken: %v", got)
	}
	// slot replacement drops fn_fulllips for fn_widemouth (same lips slot)
	if got[0] != "fn_eyebags" || got[3] != "fn_widemouth" {
		t.Fatalf("cap kept wrong tail: %v", got)
	}
}

func TestConflicts(t *testing.T) {
	cases := []struct {
		name string
		tier string
		sel  Selection
		want string // substring or "" for ok
	}{
		{"heavy+tall", "normal", Selection{"body_type": {"body_heavy"}, "height": {"h_tall"}}, "Heavy build"},
		{"heavy+very_tall", "normal", Selection{"body_type": {"body_heavy"}, "height": {"h_very_tall"}}, "Heavy build"},
		{"heavy+shortlegs freak", "freak", Selection{"body_type": {"body_heavy"}, "proportions": {"pr_shortlegs"}}, "Short legs"},
		{"muscular+longlimbs total", "total", Selection{"body_type": {"body_muscular"}, "proportions": {"pr_longlimbs"}}, "Long limbs"},
		{"heavy+shortlegs normal ok", "normal", Selection{"body_type": {"body_heavy"}, "proportions": {"pr_shortlegs"}}, ""},
		{"ultra+longneck", "total", Selection{"body_type": {"body_ultra"}, "freak_neck": {"neck_long"}}, "Long neck"},
		{"longneck+buckteeth", "total", Selection{"freak_neck": {"neck_long"}, "freak_face": {"ff_teeth_11"}}, "Buck teeth"},
		{"happy path", "total", Selection{"gender": {"male"}, "body_type": {"body_muscular"}, "height": {"h_tall"}}, ""},
	}
	for _, tc := range cases {
		_, err := ValidateSelection(tc.tier, tc.sel)
		if tc.want == "" {
			if err != nil {
				t.Fatalf("%s: unexpected error %v", tc.name, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: want error containing %q got %v", tc.name, tc.want, err)
		}
	}
}

func TestBuildPromptPreservesCharacterInTwoFrontViews(t *testing.T) {
	for _, tier := range []string{"normal", "freak", "total"} {
		t.Run(tier, func(t *testing.T) {
			p := BuildPrompt(tier, Selection{"gender": {"male"}}, "red bob haircut")
			for _, fragment := range []string{
				"16:9 landscape canvas", "exactly two side-by-side views",
				"LEFT HALF", "front-facing chest-up portrait", "RIGHT HALF",
				"front-facing full-body standing view", "highest point of the hairstyle",
				"soles of both shoes", "same single character", "selected proportions",
				"Male", "red bob haircut", "pure white background", "skin micro-texture",
			} {
				assert.Contains(t, p, fragment)
			}
			assert.NotContains(t, p, "9:16")
			assert.NotContains(t, p, "high detail, sharp focus")
			if tier == "total" {
				assert.Contains(t, p, "Extreme caricature, exaggerated proportions")
			}
		})
	}
}

func TestBuildPrompt(t *testing.T) {
	p := BuildPrompt("total", Selection{
		"gender":    {"male"},
		"hair":      {"hair_afro"},
		"skin_tone": {"st_deep"},
	}, "")
	if !strings.HasPrefix(p, "Extreme caricature, exaggerated proportions, character reference portrait of a") {
		t.Fatalf("bad lead: %q", p)
	}
	for _, frag := range []string{"Male", "afro", "deep brown skin"} {
		if !strings.Contains(p, frag) {
			t.Fatalf("missing fragment %q in %q", frag, p)
		}
	}
	if !strings.HasSuffix(p, promptTail) {
		t.Fatalf("bad suffix: %q", p)
	}
	for _, frag := range []string{"pure white background", "no gradient"} {
		if !strings.Contains(p, frag) {
			t.Fatalf("missing background constraint %q in %q", frag, p)
		}
	}
	if strings.Contains(p, "Studio background") {
		t.Fatalf("prompt must not ask for a studio backdrop: %q", p)
	}
	p2 := BuildPrompt("normal", Selection{"gender": {"female"}}, "  freckles, smiling  ")
	if !strings.Contains(p2, ". freckles, smiling.") {
		t.Fatalf("brief not appended correctly: %q", p2)
	}
}
