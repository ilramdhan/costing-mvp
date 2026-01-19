package formula

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_Evaluate_SimpleAddition(t *testing.T) {
	parser := NewParser()

	result, err := parser.Evaluate("a + b", map[string]interface{}{
		"a": 10.0,
		"b": 5.0,
	})

	require.NoError(t, err)
	assert.Equal(t, 15.0, result)
}

func TestParser_Evaluate_ComplexFormula(t *testing.T) {
	parser := NewParser()

	// Simulate real costing formula
	expression := "(electricity_kwh * rate_per_kwh) + (labor_hours * labor_rate) + overhead"
	params := map[string]interface{}{
		"electricity_kwh": 100.0,
		"rate_per_kwh":    1.5,
		"labor_hours":     8.0,
		"labor_rate":      25.0,
		"overhead":        50.0,
	}

	result, err := parser.Evaluate(expression, params)

	require.NoError(t, err)
	// 100*1.5 + 8*25 + 50 = 150 + 200 + 50 = 400
	assert.Equal(t, 400.0, result)
}

func TestParser_Evaluate_WithPercentage(t *testing.T) {
	parser := NewParser()

	// Formula with percentage calculation
	expression := "base_cost * (1 + profit_margin / 100)"
	params := map[string]interface{}{
		"base_cost":     1000.0,
		"profit_margin": 15.0,
	}

	result, err := parser.Evaluate(expression, params)

	require.NoError(t, err)
	assert.Equal(t, 1150.0, result)
}

func TestParser_Evaluate_Conditionals(t *testing.T) {
	parser := NewParser()

	// Ternary conditional in expr
	expression := "quantity > 100 ? price * 0.9 : price"
	params := map[string]interface{}{
		"quantity": 150.0,
		"price":    100.0,
	}

	result, err := parser.Evaluate(expression, params)

	require.NoError(t, err)
	assert.Equal(t, 90.0, result)
}

func TestParser_Evaluate_MissingParam(t *testing.T) {
	parser := NewParser()

	_, err := parser.Evaluate("a + b", map[string]interface{}{
		"a": 10.0,
		// "b" is missing
	})

	assert.Error(t, err)
}

func TestParser_Evaluate_InvalidExpression(t *testing.T) {
	parser := NewParser()

	// Use a syntax that is actually invalid in expr
	_, err := parser.Evaluate("((a + b", map[string]interface{}{
		"a": 10.0,
		"b": 5.0,
	})

	assert.Error(t, err)
}

func TestParser_Evaluate_TextileFormulas(t *testing.T) {
	parser := NewParser()

	testCases := []struct {
		name       string
		expression string
		params     map[string]interface{}
		expected   float64
	}{
		{
			name:       "Smelting Cost",
			expression: "(raw_material_kg * material_price) + (energy_kwh * energy_rate) + (waste_percentage * raw_material_kg * material_price)",
			params: map[string]interface{}{
				"raw_material_kg":  100.0,
				"material_price":   50.0,
				"energy_kwh":       200.0,
				"energy_rate":      1.2,
				"waste_percentage": 0.05,
			},
			expected: 5490.0, // 100*50 + 200*1.2 + 0.05*100*50 = 5000 + 240 + 250
		},
		{
			name:       "Spinning Cost",
			expression: "input_cost + (spindle_hours * spindle_rate) + (labor_hours * labor_rate)",
			params: map[string]interface{}{
				"input_cost":    5490.0,
				"spindle_hours": 10.0,
				"spindle_rate":  15.0,
				"labor_hours":   8.0,
				"labor_rate":    20.0,
			},
			expected: 5800.0, // 5490 + 10*15 + 8*20 = 5490 + 150 + 160
		},
		{
			name:       "Dyeing Cost",
			expression: "input_cost + (dye_kg * dye_price) + (water_liters * water_rate) + (steam_hours * steam_rate)",
			params: map[string]interface{}{
				"input_cost":   5800.0,
				"dye_kg":       2.5,
				"dye_price":    100.0,
				"water_liters": 500.0,
				"water_rate":   0.02,
				"steam_hours":  5.0,
				"steam_rate":   10.0,
			},
			expected: 6110.0, // 5800 + 2.5*100 + 500*0.02 + 5*10 = 5800 + 250 + 10 + 50
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parser.Evaluate(tc.expression, tc.params)
			require.NoError(t, err)
			assert.InDelta(t, tc.expected, result, 0.001)
		})
	}
}

func BenchmarkParser_Evaluate(b *testing.B) {
	parser := NewParser()
	expression := "(electricity_kwh * rate_per_kwh) + (labor_hours * labor_rate) + overhead"
	params := map[string]interface{}{
		"electricity_kwh": 100.0,
		"rate_per_kwh":    1.5,
		"labor_hours":     8.0,
		"labor_rate":      25.0,
		"overhead":        50.0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.Evaluate(expression, params)
	}
}
