package engine

import (
	"math"
	"math/rand/v2"
	"sort"
)

const (
	// neutralMultiplier is the multiplicative identity: a multiplier chain
	// with no matching modifiers leaves the score unchanged.
	neutralMultiplier = 1.0
	// neutralSum is the additive identity: an accumulator with no
	// contributions totals zero.
	neutralSum = 0.0
)

func satisfies(agent Agent, provider AffordanceProvider, action AdvertisedAction) bool {
	return preconditionsHold(agent, provider.State, action.Preconditions) &&
		costPayable(agent, action.Cost) &&
		commitmentsAllow(agent, action)
}

func preconditionsHold(agent Agent, providerState string, preconditions []Precondition) bool {
	for _, precondition := range preconditions {
		if precondition.Capability != "" && !agent.Capabilities[precondition.Capability] {
			return false
		}
		if precondition.Resource != "" && agent.Resources[precondition.Resource] < precondition.Minimum {
			return false
		}
		if precondition.RequiredState != "" && providerState != precondition.RequiredState {
			return false
		}
	}
	return true
}

func costPayable(agent Agent, cost map[string]float64) bool {
	for resourceID, amount := range cost {
		if agent.Resources[resourceID] < amount {
			return false
		}
	}
	return true
}

func commitmentsAllow(agent Agent, action AdvertisedAction) bool {
	for _, commitment := range agent.NarrativeCommitments {
		if commitmentContradicts(commitment, action.Tags) {
			return false
		}
	}
	return true
}

func commitmentContradicts(commitment NarrativeCommitment, tags []string) bool {
	for _, tag := range tags {
		if commitment.ContradictoryTags[tag] {
			return true
		}
	}
	return false
}

func tagMultiplier(modifiers map[string]float64, tags []string) float64 {
	multiplier := neutralMultiplier
	for _, tag := range tags {
		if modifier, exists := modifiers[tag]; exists {
			multiplier *= modifier
		}
	}
	return multiplier
}

func personalityMultiplier(agent Agent, action AdvertisedAction) float64 {
	multiplier := neutralMultiplier
	for _, trait := range agent.Personality.Traits {
		multiplier *= tagMultiplier(trait.Modifiers, action.Tags)
	}
	if preference, ok := agent.Personality.Preferences[action.Domain]; ok {
		multiplier *= 1 + preference/10
	}
	return multiplier
}

func contextMultiplier(agent Agent, action AdvertisedAction) float64 {
	multiplier := neutralMultiplier
	for _, context := range agent.ActiveContexts {
		contextTagMultiplier := tagMultiplier(context.Modifiers, action.Tags)
		// This intentional salience boost squares the context multiplier. A
		// single application is diluted by softmax, so the observable frequency
		// of tagged actions does not double as intended. Do not reduce this to
		// one factor without recalibrating the behavior.
		multiplier *= contextTagMultiplier * contextTagMultiplier
	}
	return multiplier
}

func distanceMultiplier(agent Agent, provider AffordanceProvider, tuning Tuning) float64 {
	distanceReference := tuning.DistanceReference
	if distanceReference <= 0 {
		distanceReference = DISTANCE_REFERENCE
	}
	return 1 / (1 + agent.Position.Distance(provider.Position)/distanceReference)
}

func weightedCost(agent Agent, action AdvertisedAction) float64 {
	penalty := neutralSum
	for resourceID, cost := range action.Cost {
		availableAmount := agent.Resources[resourceID]
		penalty += cost / math.Max(1, availableAmount/cost)
	}
	return penalty
}

// ScoreAction scores an advertised promise. Personality, context, and distance scale benefits;
// costs remain penalties so a cheap action cannot become expensive through a positive modifier.
func ScoreAction(agent Agent, provider AffordanceProvider, action AdvertisedAction, tuning Tuning) float64 {
	considerationBenefit := neutralSum
	defaultResponseCurveExponent := responseCurveExponent(tuning)
	for considerationID, delta := range action.Deltas {
		consideration, exists := agent.Considerations[considerationID]
		if exists {
			expectedGain := math.Min(delta, consideration.Max-consideration.Value)
			considerationBenefit += pressure(consideration, defaultResponseCurveExponent) * expectedGain
		}
	}

	priorityWeight := tuning.WPriority
	if priorityWeight == 0 {
		priorityWeight = W_PRIORITY
	}
	considerationBenefit += action.IntrinsicPriority * priorityWeight

	return considerationBenefit*
		personalityMultiplier(agent, action)*
		contextMultiplier(agent, action)*
		distanceMultiplier(agent, provider, tuning) -
		weightedCost(agent, action)
}

// Candidates returns all eligible actions in descending utility order.
func Candidates(agent Agent, providers []AffordanceProvider, tuning Tuning) []Candidate {
	candidates := make([]Candidate, 0)
	for _, provider := range providers {
		if provider.Capacity-len(provider.Occupants) <= 0 {
			continue
		}

		for _, action := range provider.AdvertisedActions {
			if actionSaturated(action) || !isActionInRange(agent, provider, action) || !satisfies(agent, provider, action) {
				continue
			}

			candidates = append(candidates, Candidate{
				Action:   action,
				Provider: provider,
				Utility:  ScoreAction(agent, provider, action, tuning),
			})
		}
	}
	sort.SliceStable(candidates, func(leftIndex, rightIndex int) bool {
		return candidates[leftIndex].Utility > candidates[rightIndex].Utility
	})
	return candidates
}

func isActionInRange(agent Agent, provider AffordanceProvider, action AdvertisedAction) bool {
	return provider.Position.Distance(agent.Position) <= action.AdvertisementRadius
}

// actionSaturated reports whether the action's own slots are already consumed
// by the occupancy the client reported. Capacity 0 means unlimited — see the
// warning on AdvertisedAction.Capacity.
func actionSaturated(action AdvertisedAction) bool {
	return action.Capacity > 0 && action.Occupancy >= action.Capacity
}

// SelectAction selects one of the highest-scoring candidates with softmax.
func SelectAction(agent Agent, providers []AffordanceProvider, tuning Tuning, rng *rand.Rand) *ActionInstance {
	var candidates []Candidate
	if tuning.PerceptionNoise > 0 {
		candidates = Candidates(perceivedAgent(agent, tuning.PerceptionNoise, rng), providers, tuning)
	} else {
		candidates = Candidates(agent, providers, tuning)
	}
	if len(candidates) == 0 {
		return nil
	}

	temperature := tuning.SelectionTemperature
	if temperature <= 0 {
		temperature = SELECTION_TEMPERATURE
	}
	selected := sampleSoftmax(topCandidates(candidates, tuning.SelectionTopK), temperature, rng)
	if tuning.ConventionBreakProbability > 0 && shouldBreakConvention(tuning.ConventionBreakProbability, rng) {
		if contrarian, ok := conventionBreakingCandidate(candidates, rng); ok {
			selected = contrarian
		}
	}
	return &ActionInstance{
		Action:              selected.Action,
		ProviderID:          selected.Provider.ID,
		ContinuationUtility: selected.Utility,
	}
}

func perceivedAgent(agent Agent, noise float64, rng *rand.Rand) Agent {
	perceived := agent
	perceived.Considerations = make(map[string]Consideration, len(agent.Considerations))
	considerationIDs := make([]string, 0, len(agent.Considerations))
	for considerationID, consideration := range agent.Considerations {
		perceived.Considerations[considerationID] = consideration
		considerationIDs = append(considerationIDs, considerationID)
	}
	sort.Strings(considerationIDs)
	for _, considerationID := range considerationIDs {
		consideration := perceived.Considerations[considerationID]
		if consideration.Updater.Kind != PerceptionDriven {
			continue
		}
		consideration.Value = clamp(consideration.Value+rng.NormFloat64()*noise, consideration.Min, consideration.Max)
		perceived.Considerations[considerationID] = consideration
	}
	return perceived
}

func shouldBreakConvention(probability float64, rng *rand.Rand) bool {
	return probability > 0 && rng.Float64() < probability
}

func conventionBreakingCandidate(candidates []Candidate, rng *rand.Rand) (Candidate, bool) {
	bestUtility := candidates[0].Utility
	firstLowerUtility := 0
	for firstLowerUtility < len(candidates) && candidates[firstLowerUtility].Utility >= bestUtility {
		firstLowerUtility++
	}
	if firstLowerUtility == len(candidates) {
		return Candidate{}, false
	}
	return candidates[firstLowerUtility+rng.IntN(len(candidates)-firstLowerUtility)], true
}

func topCandidates(candidates []Candidate, configuredTopK int) []Candidate {
	topK := configuredTopK
	if topK <= 0 {
		topK = SELECTION_TOP_K
	}
	if topK > len(candidates) {
		topK = len(candidates)
	}
	return candidates[:topK]
}

func sampleSoftmax(candidates []Candidate, temperature float64, rng *rand.Rand) Candidate {
	maxUtility := candidates[0].Utility
	totalWeight := neutralSum
	weights := make([]float64, len(candidates))
	for candidateIndex, candidate := range candidates {
		weights[candidateIndex] = math.Exp((candidate.Utility - maxUtility) / temperature)
		totalWeight += weights[candidateIndex]
	}

	draw := rng.Float64() * totalWeight
	selected := candidates[len(candidates)-1]
	for candidateIndex, weight := range weights {
		draw -= weight
		if draw <= 0 {
			selected = candidates[candidateIndex]
			break
		}
	}
	return selected
}
