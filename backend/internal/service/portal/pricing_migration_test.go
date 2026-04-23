package portal

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"higress-portal-backend/internal/model"
)

func TestBootstrapCatalogPricingUsesPerMillionUnits(t *testing.T) {
	pricing := bootstrapCatalogPricing(123.45, 678.9)

	require.Equal(t, billingCurrencyCNY, pricing.Currency)
	require.Equal(t, 123.45, pricing.InputCostPerMillionTokens)
	require.Equal(t, 678.9, pricing.OutputCostPerMillionTokens)
	require.EqualValues(t, 123, rmbPerMillionToMicroYuanPerToken(pricing.InputCostPerMillionTokens))
	require.EqualValues(t, 679, rmbPerMillionToMicroYuanPerToken(pricing.OutputCostPerMillionTokens))
}

func TestEnsureBillingModelPriceColumnsUsesDriverSpecificDDL(t *testing.T) {
	svc := &Service{}

	postgresSQL := svc.sqlForDriver(
		`ALTER TABLE billing_model_price_version ADD COLUMN input_price_micro_yuan_per_token BIGINT NOT NULL DEFAULT 0`,
	)
	supportsPromptCachingSQL := svc.sqlForDriver(
		`ALTER TABLE billing_model_price_version ADD COLUMN supports_prompt_caching BOOLEAN NOT NULL DEFAULT FALSE`,
	)

	require.NotContains(t, postgresSQL, "AFTER")
	require.True(t, strings.Contains(postgresSQL, "ADD COLUMN input_price_micro_yuan_per_token"))
	require.NotContains(t, supportsPromptCachingSQL, "AFTER")
	require.True(t, strings.Contains(supportsPromptCachingSQL, "BOOLEAN NOT NULL DEFAULT FALSE"))
}

func TestMaterializeModelPricingForImageGenerationMirrorsOutputImagePrice(t *testing.T) {
	pricing := materializeModelPricingForType("image_generation", model.ModelPricing{
		Currency:      billingCurrencyCNY,
		PricePerImage: 0.2,
	})

	require.Equal(t, billingCurrencyCNY, pricing.Currency)
	require.Equal(t, 0.2, pricing.PricePerImage)
	require.Equal(t, 0.2, pricing.OutputCostPerImage)
}
