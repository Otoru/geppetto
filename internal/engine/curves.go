package engine

import "math"

// Evaluate converts value into the normalized pressure defined by r. A convex
// curve without an explicit exponent resolves through the same default path
// as the scoring pipeline, with an empty tuning.
func (r ResponseCurve) Evaluate(value, minimum, maximum float64) float64 {
	return r.evaluate(value, minimum, maximum, responseCurveExponent(Tuning{}))
}

func (r ResponseCurve) evaluate(value, minimum, maximum, defaultExponent float64) float64 {
	if maximum <= minimum {
		return 0
	}

	switch r.Kind {
	case Convex:
		exponent := r.Exponent
		if exponent == 0 {
			// Profiles establish the default; an explicit curve exponent is a
			// deliberate per-consideration override.
			exponent = defaultExponent
		}
		normalized := clamp((maximum-value)/(maximum-minimum), 0, 1)
		return math.Pow(normalized, exponent)
	case Linear:
		return clamp((maximum-value)/(maximum-minimum), 0, 1)
	case Step:
		if value < r.Threshold {
			return r.Below
		}
		return r.Above
	case Logistic:
		slope := r.Slope
		if slope == 0 {
			slope = 1
		}
		return 1 / (1 + math.Exp(-slope*(value-r.Midpoint)))
	default:
		return 0
	}
}

func clamp(value, minimum, maximum float64) float64 {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func pressure(consideration Consideration, defaultExponent float64) float64 {
	return consideration.BaseWeight * consideration.ResponseCurve.evaluate(
		consideration.Value, consideration.Min, consideration.Max,
		defaultExponent,
	)
}

// responseCurveExponent resolves the profile default for convex curves; a
// zero tuning value means "not configured" and falls back to the engine
// default.
func responseCurveExponent(tuning Tuning) float64 {
	if tuning.ResponseCurveExponent != 0 {
		return tuning.ResponseCurveExponent
	}
	return RESPONSE_CURVE_EXPONENT
}
