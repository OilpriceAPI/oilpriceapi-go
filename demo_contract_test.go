package oilpriceapi

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"
)

var coreDemoCodes = []string{
	"BRENT_CRUDE_USD",
	"WTI_USD",
	"NATURAL_GAS_USD",
	"GOLD_USD",
	"EUR_USD",
	"GBP_USD",
	"HEATING_OIL_USD",
	"GASOLINE_USD",
	"DIESEL_USD",
}

func validateDemoCompatibility(prices []DemoPrice) error {
	required := make(map[string]struct{}, len(coreDemoCodes))
	for _, code := range coreDemoCodes {
		required[code] = struct{}{}
	}

	seen := make(map[string]struct{}, len(prices))
	for _, price := range prices {
		if strings.TrimSpace(price.Code) == "" ||
			strings.TrimSpace(price.Name) == "" ||
			strings.TrimSpace(price.Currency) == "" ||
			strings.TrimSpace(price.UpdatedAt) == "" ||
			math.IsNaN(price.Price) || math.IsInf(price.Price, 0) || price.Price <= 0 {
			return fmt.Errorf("demo returned an unusable price row: %+v", price)
		}
		if _, duplicate := seen[price.Code]; duplicate {
			return fmt.Errorf("demo returned duplicate code %q", price.Code)
		}
		seen[price.Code] = struct{}{}
		delete(required, price.Code)
	}
	if len(required) > 0 {
		missing := make([]string, 0, len(required))
		for code := range required {
			missing = append(missing, code)
		}
		sort.Strings(missing)
		return fmt.Errorf("demo compatibility floor is missing core codes: %s", strings.Join(missing, ", "))
	}
	return nil
}

func TestDemoCompatibilityRejectsCoreCodeSubstitution(t *testing.T) {
	prices := validCoreDemoPrices()
	prices[1] = DemoPrice{
		Code:      "REPLACEMENT_USD",
		Name:      "Replacement",
		Price:     1,
		Currency:  "USD",
		UpdatedAt: "2026-08-11T00:00:00Z",
	}
	err := validateDemoCompatibility(prices)
	if err == nil || !strings.Contains(err.Error(), "WTI_USD") {
		t.Fatalf("expected a missing-WTI failure, got %v", err)
	}
}

func TestDemoCompatibilityValidatesUsableUniqueRows(t *testing.T) {
	if err := validateDemoCompatibility(validCoreDemoPrices()); err != nil {
		t.Fatalf("valid core rejected: %v", err)
	}

	t.Run("timestamp", func(t *testing.T) {
		prices := validCoreDemoPrices()
		prices[0].UpdatedAt = ""
		if err := validateDemoCompatibility(prices); err == nil || !strings.Contains(err.Error(), "unusable") {
			t.Fatalf("expected unusable-row failure, got %v", err)
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		prices := validCoreDemoPrices()
		prices = append(prices, prices[0])
		if err := validateDemoCompatibility(prices); err == nil || !strings.Contains(err.Error(), "duplicate") {
			t.Fatalf("expected duplicate-code failure, got %v", err)
		}
	})
}

func validCoreDemoPrices() []DemoPrice {
	prices := make([]DemoPrice, 0, len(coreDemoCodes))
	for _, code := range coreDemoCodes {
		prices = append(prices, DemoPrice{
			Code:      code,
			Name:      code,
			Price:     1,
			Currency:  "USD",
			UpdatedAt: "2026-08-11T00:00:00Z",
		})
	}
	return prices
}
