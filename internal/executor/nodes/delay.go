package nodes

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// DelayNode handles workflow delay/pause
type DelayNode struct{}

// NewDelayNode creates a new delay node executor
func NewDelayNode() *DelayNode {
	return &DelayNode{}
}

// Execute executes a delay node
func (n *DelayNode) Execute(ctx context.Context, input NodeInput) (NodeOutput, error) {
	if input.Config == nil {
		return NodeOutput{}, errors.New("delay config is required")
	}

	// Extract delay configuration
	duration, ok := input.Config["duration"].(float64)
	if !ok || duration <= 0 {
		return NodeOutput{}, errors.New("delay duration must be a positive number")
	}

	unit, ok := input.Config["unit"].(string)
	if !ok {
		unit = "seconds" // default unit
	}

	// Convert duration to time.Duration based on unit
	var delayDuration time.Duration
	switch unit {
	case "milliseconds":
		delayDuration = time.Duration(duration) * time.Millisecond
	case "seconds":
		delayDuration = time.Duration(duration) * time.Second
	case "minutes":
		delayDuration = time.Duration(duration) * time.Minute
	case "hours":
		delayDuration = time.Duration(duration) * time.Hour
	default:
		return NodeOutput{}, fmt.Errorf("unsupported time unit: %s", unit)
	}

	// Execute the delay with context cancellation support
	select {
	case <-time.After(delayDuration):
		// Delay completed successfully
		output := NodeOutput{
			Success: true,
			Data: map[string]interface{}{
				"delayed":    true,
				"duration":  duration,
				"unit":       unit,
				"actualDelay": delayDuration.String(),
			},
		}
		return output, nil
	case <-ctx.Done():
		// Context was cancelled
		return NodeOutput{}, errors.New("delay was cancelled")
	}
}

// Validate validates delay node configuration
func (n *DelayNode) Validate(config map[string]interface{}) error {
	if config == nil {
		return errors.New("delay config is required")
	}

	duration, ok := config["duration"].(float64)
	if !ok || duration <= 0 {
		return errors.New("delay duration must be a positive number")
	}

	unit, ok := config["unit"].(string)
	if !ok {
		return errors.New("delay unit is required")
	}

	validUnits := map[string]bool{
		"milliseconds": true,
		"seconds":      true,
		"minutes":      true,
		"hours":        true,
	}

	if !validUnits[unit] {
		return fmt.Errorf("invalid time unit: %s. Must be one of: milliseconds, seconds, minutes, hours", unit)
	}

	return nil
}

// GetConfigSchema returns the expected configuration schema for delay nodes
func (n *DelayNode) GetConfigSchema() map[string]interface{} {
	return map[string]interface{}{
		"duration": map[string]interface{}{
			"type":        "number",
			"description": "Delay duration",
			"required":    true,
			"minimum":     0,
		},
		"unit": map[string]interface{}{
			"type":        "string",
			"description": "Time unit for duration",
			"required":    true,
			"enum":        []string{"milliseconds", "seconds", "minutes", "hours"},
			"default":     "seconds",
		},
	}
}

func (n *DelayNode) String() string {
	return fmt.Sprintf("DelayNode")
}