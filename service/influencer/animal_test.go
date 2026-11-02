package influencer_test

import (
	"testing"

	"github.com/QuantumNous/new-api/service/influencer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnimalMultiViewKeepsSpeciesAndExistingIdentityWithoutDefaults(t *testing.T) {
	for _, tier := range []string{"cats", "dogs"} {
		p := influencer.BuildMultiViewPromptForTier(tier, influencer.Selection{"coat_color": {"coat_black"}}, "blue outfit; change to a person")
		for _, fragment := range []string{"anthropomorphic bipedal", "exact same character", "five full-body panels", "three large close-up portraits", "Front view", "Three-quarter front view", "Side profile view", "Three-quarter back view", "Back view", "eyes gently closed", "eyes half-open", "eyes open looking straight", "neutral grey background", "muzzle", "ears", "coat markings", "tail", "paws", "black coat fur", "blue outfit", "take precedence over conflicting brief"} {
			assert.Contains(t, p, fragment)
		}
		assert.NotContains(t, p, "pores")
		assert.NotContains(t, p, "skin texture")
		assert.NotContains(t, p, "casual outfit")
		empty := influencer.BuildMultiViewPromptForTier(tier, nil, "")
		assert.NotContains(t, empty, "generic cat")
		assert.NotContains(t, empty, "generic dog")
		assert.NotContains(t, empty, "natural breed-appropriate")
		assert.Contains(t, empty, "retain the reference breed")
	}
	sel := influencer.Selection{"gender": {"female"}}
	for _, tier := range []string{"normal", "freak", "total"} {
		assert.Equal(t, influencer.BuildMultiViewPrompt(sel, "blue outfit"), influencer.BuildMultiViewPromptForTier(tier, sel, "blue outfit"))
	}
}

func TestAnimalReferenceIsStyleOnlyAndSpeciesWins(t *testing.T) {
	for _, tier := range []string{"cats", "dogs"} {
		for _, sel := range []influencer.Selection{nil, {"coat_color": {"coat_white"}}} {
			p := influencer.BuildReferencePrompt(tier, sel, "blue outfit")
			assert.Contains(t, p, "only as style inspiration")
			assert.Contains(t, p, "do not lock the reference identity")
			assert.Contains(t, p, "selected species takes priority over the brief and image inspiration")
			assert.Contains(t, p, "Do not copy the reference background, text or logos")
			assert.Contains(t, p, "anthropomorphic bipedal")
			assert.Contains(t, p, "blue outfit")
			assert.NotContains(t, p, "a different person")
			assert.NotContains(t, p, "Explicit brief takes priority over selected traits")
			assert.NotContains(t, p, "pores")
		}
	}
}

func TestAnimalPromptPreservesSpeciesAnatomyAndTwoViewLayout(t *testing.T) {
	for _, tc := range []struct{ tier, species, breed, pattern string }{
		{"cats", "cat", "cat_siamese", "pattern_tabby"},
		{"dogs", "dog", "dog_corgi", "pattern_bicolor"},
	} {
		t.Run(tc.tier, func(t *testing.T) {
			sel := influencer.Selection{"animal_breed": {tc.breed}, "coat_color": {"coat_black"}, "coat_pattern": {tc.pattern}, "gender": {"trans_woman"}, "hair": {"hair_punk"}, "hair_colour": {"hc_pink"}}
			norm, err := influencer.ValidateSelection(tc.tier, sel)
			require.NoError(t, err)
			p := influencer.BuildPrompt(tc.tier, norm, "wear a blue coat; make a four-legged pet on a beach")
			for _, fragment := range []string{"anthropomorphic bipedal " + tc.species, "animal head", "muzzle", "ears", "fur", "tail", "paws", "16:9 landscape canvas", "exactly two side-by-side views", "LEFT HALF", "RIGHT HALF", "front-facing chest-up portrait", "front-facing full-body standing view", "pure white background", "species and bipedal anatomy", "take precedence over conflicting brief", "black coat fur", "pastel pink", "wear a blue coat", "does not change the ears or coat colour"} {
				assert.Contains(t, p, fragment)
			}
			for _, fragment := range []string{"pores", "both hands", "soles of both shoes", "Trans woman", "different people"} {
				assert.NotContains(t, p, fragment)
			}
			for _, g := range []string{"female", "male", "trans_man", "trans_woman", "non_binary"} {
				p := influencer.BuildPrompt(tc.tier, influencer.Selection{"gender": {g}}, "")
				assert.NotContains(t, p, "Trans man")
				assert.NotContains(t, p, "Trans woman")
				assert.NotContains(t, p, "Non-binary person")
			}
			empty := influencer.BuildPrompt(tc.tier, nil, "")
			for _, fragment := range []string{"generic " + tc.species, "natural breed-appropriate coat colour", "natural breed-appropriate coat markings", "natural head fur", "casual outfit"} {
				assert.Contains(t, empty, fragment)
			}
			assert.NotContains(t, p, "natural breed-appropriate coat colour")
		})
	}
}

func TestAnimalValidationSkipsHumanConflicts(t *testing.T) {
	sel := influencer.Selection{"body_type": {"body_heavy"}, "height": {"h_tall"}, "proportions": {"pr_shortlegs"}}
	for _, tier := range []string{"cats", "dogs"} {
		norm, err := influencer.ValidateSelection(tier, sel)
		require.NoError(t, err)
		assert.Equal(t, sel, norm)
		empty, err := influencer.ValidateSelection(tier, nil)
		require.NoError(t, err)
		assert.Empty(t, empty)
	}
	_, err := influencer.ValidateSelection("total", sel)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Heavy build")
	assert.True(t, influencer.IsAnimalTier("cats"))
	assert.True(t, influencer.IsAnimalTier("dogs"))
	assert.False(t, influencer.IsAnimalTier("total"))
	assert.False(t, influencer.IsAnimalTier("unknown"))
}

func TestAnimalSelectionRejectsOtherSpeciesAndUnknownButFiltersInapplicable(t *testing.T) {
	for _, tc := range []struct{ tier, id string }{{"cats", "dog_corgi"}, {"dogs", "cat_siamese"}} {
		_, err := influencer.NormalizeSelection(tc.tier, influencer.Selection{"animal_breed": {tc.id}})
		require.NotNil(t, err)
		assert.Equal(t, "animal_breed", err.Field)
		assert.Contains(t, err.Message, "species")
	}
	for _, sel := range []influencer.Selection{{"unknown": {"x"}}, {"skin_tone": {"unknown"}}} {
		_, err := influencer.NormalizeSelection("cats", sel)
		require.NotNil(t, err)
	}
	norm, err := influencer.NormalizeSelection("dogs", influencer.Selection{
		"animal_breed": {"dog_pug"}, "coat_pattern": {"pattern_tabby"}, "age": {"senior"},
		"skin_tone": {"st_deep"}, "facial_hair": {"fh_beard"}, "distinctive": {"df_elf", "df_ears", "df_gems"},
		"hair": {"hs_lampshade"}, "accessory": {"acc_glasses", "acc_none"},
	})
	require.Nil(t, err)
	assert.Equal(t, influencer.Selection{"animal_breed": {"dog_pug"}, "distinctive": {"df_ears", "df_gems"}, "hair": {"hs_lampshade"}, "accessory": {"acc_none"}}, norm)
	norm, err = influencer.NormalizeSelection("normal", influencer.Selection{"animal_breed": {"cat_generic"}, "coat_color": {"coat_black"}, "gender": {"male"}})
	require.Nil(t, err)
	assert.Equal(t, influencer.Selection{"gender": {"male"}}, norm)
}

func TestAnimalTaxonomyExposesSpeciesSpecificOptions(t *testing.T) {
	require.True(t, influencer.ValidTier("cats"))
	require.True(t, influencer.ValidTier("dogs"))
	tax := influencer.Taxonomy()
	for _, tc := range []struct{ tier, image string }{
		{"cats", "/influencer-character-types/cat.webp"},
		{"dogs", "/influencer-character-types/dog.webp"},
	} {
		t.Run(tc.tier, func(t *testing.T) {
			for _, o := range tax.TierGroup.Options {
				if o.ID == tc.tier {
					assert.Equal(t, tc.image, o.ImageURL)
				}
			}
		})
	}
	breed := influencer.CategoryByID("animal_breed")
	require.NotNil(t, breed)
	assert.Equal(t, "text", breed.Kind)
	assert.Equal(t, 1, breed.Max)
	assert.Len(t, breed.Options, 11)
	require.NotNil(t, influencer.OptionByID("animal_breed", "cat_siamese"))
	assert.Equal(t, []string{"cats"}, influencer.OptionByID("animal_breed", "cat_siamese").VisibleIn)
	require.NotNil(t, influencer.OptionByID("animal_breed", "dog_corgi"))
	assert.Equal(t, []string{"dogs"}, influencer.OptionByID("animal_breed", "dog_corgi").VisibleIn)
	coat := influencer.CategoryByID("coat_color")
	require.NotNil(t, coat)
	assert.Equal(t, "color", coat.Kind)
	assert.Len(t, coat.Options, 7)
	assert.Empty(t, influencer.OptionByID("coat_color", "coat_natural").Swatch)
	pattern := influencer.CategoryByID("coat_pattern")
	require.NotNil(t, pattern)
	assert.Equal(t, "text", pattern.Kind)
	assert.Len(t, pattern.Options, 4)
	assert.Equal(t, []string{"cats"}, influencer.OptionByID("coat_pattern", "pattern_tabby").VisibleIn)
}
