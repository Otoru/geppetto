package engine

import "math/rand/v2"

// CheckPreemption replaces the current action when a critical consideration
// makes the best available action valuable enough to clear the configured margin.
func CheckPreemption(agent *Agent, providers []AffordanceProvider, tuning Tuning, rng *rand.Rand) bool {
	if agent.CurrentAction == nil {
		return false
	}

	hasCriticalConsideration, hasImminentCollapse := urgencyState(agent.Considerations)
	if !hasCriticalConsideration || (agent.CurrentAction.PlayerQueued && !hasImminentCollapse) {
		return false
	}

	bestAction := SelectAction(*agent, providers, tuning, rng)
	if bestAction == nil {
		return false
	}

	preemptionMargin := tuning.PreemptionMargin
	if preemptionMargin == 0 {
		preemptionMargin = 1.5
	}
	if bestAction.ContinuationUtility > preemptionMargin*agent.CurrentAction.ContinuationUtility {
		agent.CurrentAction = bestAction
		return true
	}
	return false
}

func urgencyState(considerations map[string]Consideration) (hasCriticalConsideration, hasImminentCollapse bool) {
	for _, consideration := range considerations {
		if consideration.Value < consideration.CriticalThreshold {
			hasCriticalConsideration = true
		}
		if consideration.Value <= -90 {
			hasImminentCollapse = true
		}
	}
	return hasCriticalConsideration, hasImminentCollapse
}

// Tick advances one full-detail simulation tick for agent.
func Tick(agent *Agent, providers []AffordanceProvider, world World, deltaHours float64, tuning Tuning, rng *rand.Rand) {
	UpdateConsiderations(agent, world, deltaHours)
	advanceCurrentAction(agent, deltaHours)

	if agent.CurrentAction == nil && len(agent.ActionQueue) == 0 {
		agent.CurrentAction = SelectAction(*agent, providers, tuning, rng)
		return
	}

	CheckPreemption(agent, providers, tuning, rng)
}

func advanceCurrentAction(agent *Agent, deltaHours float64) {
	if agent.CurrentAction == nil {
		return
	}

	ApplyGradualDeltas(agent, agent.CurrentAction, deltaHours)
	if agent.CurrentAction.Elapsed >= agent.CurrentAction.Action.EstimatedDuration {
		agent.CurrentAction = nil
	}
}

// ApplyAggregatedEvent records an event produced while an agent uses simplified LOD.
func ApplyAggregatedEvent(agent *Agent, event AggregatedEvent) {
	agent.AggregatedEvents = append(agent.AggregatedEvents, event)
}

// TransitionToFull reconstructs a plausible detailed state from accumulated events.
func TransitionToFull(agent *Agent, hour float64) {
	for _, event := range agent.AggregatedEvents {
		if event.Kind == "meal" && event.AtHour <= hour {
			applyMealReconstruction(agent)
		}
	}
	agent.AggregatedEvents = nil
	agent.SimulationLevel = Full
}

func applyMealReconstruction(agent *Agent) {
	hunger, exists := agent.Considerations["HUNGER"]
	if !exists {
		return
	}

	hunger.Value = clamp(hunger.Value+120, hunger.Min, hunger.Max)
	agent.Considerations["HUNGER"] = hunger
}
