package formula

import (
	"fmt"

	"github.com/expr-lang/expr"
)

// Parser handles formula parsing and evaluation
type Parser struct {
	// No cache needed since we compile with params each time
}

// NewParser creates a new formula parser
func NewParser() *Parser {
	return &Parser{}
}

// Evaluate evaluates a formula with given parameters
func (p *Parser) Evaluate(expression string, params map[string]interface{}) (float64, error) {
	// Compile with the actual parameters as the environment
	program, err := expr.Compile(expression, expr.Env(params), expr.AsFloat64())
	if err != nil {
		return 0, fmt.Errorf("failed to compile expression '%s': %w", expression, err)
	}

	result, err := expr.Run(program, params)
	if err != nil {
		return 0, fmt.Errorf("failed to evaluate formula: %w", err)
	}

	switch v := result.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("unexpected result type: %T", result)
	}
}

// ValidateExpression validates a formula expression with sample params
func (p *Parser) ValidateExpression(expression string, sampleParams map[string]interface{}) error {
	_, err := expr.Compile(expression, expr.Env(sampleParams))
	return err
}

// DefaultParser is the global parser instance
var DefaultParser = NewParser()

// Evaluate is a convenience function using the default parser
func Evaluate(expression string, params map[string]interface{}) (float64, error) {
	return DefaultParser.Evaluate(expression, params)
}
