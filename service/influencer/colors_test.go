package influencer

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestExpandedColorsCanBeSelectedInEveryHumanTier(t *testing.T) {
	hair := CategoryByID("hair_colour")
	skin := CategoryByID("skin_tone")
	require.NotNil(t, hair)
	require.NotNil(t, skin)
	assert.Len(t, hair.Options, 28)
	assert.Len(t, skin.Options, 9)
	for _, category := range []*Category{hair, skin} {
		for _, option := range category.Options {
			for _, tier := range []string{"normal", "freak", "total"} {
				selection, err := NormalizeSelection(tier, Selection{category.ID: {option.ID}})
				require.Nil(t, err)
				assert.Equal(t, []string{option.ID}, selection[category.ID])
				assert.NotEqual(t, option.ID, Fragment(category.ID, option.ID))
			}
		}
	}
	carbon := OptionByID("skin_tone", "st_carbon")
	require.NotNil(t, carbon)
	assert.Equal(t, "#181412", carbon.Swatch)
	assert.Nil(t, OptionByID("skin_tone", "st_honey"))
}
