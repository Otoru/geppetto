package geppetto

import "math/rand/v2"

// TickRequest describes one in-process Advance call. NowHours is the absolute
// simulation hour at which expiry and aggregate-event timestamps are evaluated.
// TargetLevel is selected by the integrating simulation; an empty value keeps
// the agent's current level (or Full for a new agent). AggregatedEvent is
// recorded only while the target level is Simplified.
type TickRequest struct {
	World           World
	NowHours        float64
	TargetLevel     SimulationLevel
	AggregatedEvent *AggregatedEvent
}

// TickResult reports the effective level and simulated duration of one Advance
// call. The caller schedules its next advance after ElapsedHours.
type TickResult struct {
	SimulationLevel SimulationLevel
	ElapsedHours    float64
}

// Advance runs one complete simulation step. It first expires commitments so
// expired restrictions cannot block this step. A simplified step records its
// optional coarse event and returns its aggregate cadence. A transition back
// to Full reconstructs accumulated events before the detailed tick updates
// considerations, applies the current action, then either starts queued work
// or selects a new action for an idle agent, or evaluates preemption for a
// continuing action. This order lets the decision see the state produced by
// the elapsed interval while a newly started action begins contributing on the
// following interval.
func Advance(agent *Agent, profile Profile, providers []AffordanceProvider, request TickRequest, rng *rand.Rand) TickResult {
	expireNarrativeCommitments(agent, request.NowHours)

	switch targetSimulationLevel(agent.SimulationLevel, request.TargetLevel) {
	case Simplified:
		agent.SimulationLevel = Simplified
		if request.AggregatedEvent != nil {
			event := *request.AggregatedEvent
			event.AtHour = request.NowHours
			ApplyAggregatedEvent(agent, event)
		}
		return TickResult{SimulationLevel: Simplified, ElapsedHours: simplifiedTickHours(profile.Tuning)}
	default:
		if agent.SimulationLevel == Simplified {
			TransitionToFull(agent, request.NowHours, profile.AggregatedEventEffects)
		} else {
			agent.SimulationLevel = Full
		}
		Tick(agent, providers, request.World, fullTickHours(profile.Tuning), profile.Tuning, rng)
		return TickResult{SimulationLevel: Full, ElapsedHours: fullTickHours(profile.Tuning)}
	}
}

func targetSimulationLevel(current, requested SimulationLevel) SimulationLevel {
	if requested != "" {
		return requested
	}
	if current == Simplified {
		return Simplified
	}
	return Full
}

func fullTickHours(tuning Tuning) float64 {
	if tuning.FullTickHours > 0 {
		return tuning.FullTickHours
	}
	return fullTickHoursDefault
}

func simplifiedTickHours(tuning Tuning) float64 {
	if tuning.SimplifiedTickHours > 0 {
		return tuning.SimplifiedTickHours
	}
	return simplifiedTickHoursDefault
}

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
		preemptionMargin = preemptionMarginDefault
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
		if consideration.Value <= collapseThreshold(consideration) {
			hasImminentCollapse = true
		}
	}
	return hasCriticalConsideration, hasImminentCollapse
}

// collapseThreshold reads the imminent-collapse floor from the consideration
// itself; a zero CriticalThreshold means "not configured" and falls back to
// the engine default.
func collapseThreshold(consideration Consideration) float64 {
	if consideration.CriticalThreshold != 0 {
		return consideration.CriticalThreshold
	}
	return imminentCollapseThreshold
}

// Tick advances one detailed interval after Advance has chosen Full.
func Tick(agent *Agent, providers []AffordanceProvider, world World, deltaHours float64, tuning Tuning, rng *rand.Rand) {
	UpdateConsiderations(agent, world, deltaHours)
	advanceCurrentAction(agent, deltaHours)

	if agent.CurrentAction == nil {
		if dequeueAction(agent) {
			return
		}
		agent.CurrentAction = SelectAction(*agent, providers, tuning, rng)
		return
	}

	CheckPreemption(agent, providers, tuning, rng)
}

// EnqueueAction appends an integrating game's explicit action request. Queued
// actions are FIFO and protected from ordinary preemption after they start.
func EnqueueAction(agent *Agent, action ActionInstance) {
	action.PlayerQueued = true
	agent.ActionQueue = append(agent.ActionQueue, action)
}

func dequeueAction(agent *Agent) bool {
	if len(agent.ActionQueue) == 0 {
		return false
	}
	next := agent.ActionQueue[0]
	agent.ActionQueue[0] = ActionInstance{}
	agent.ActionQueue = agent.ActionQueue[1:]
	agent.CurrentAction = &next
	return true
}

func expireNarrativeCommitments(agent *Agent, nowHours float64) {
	active := agent.NarrativeCommitments[:0]
	for _, commitment := range agent.NarrativeCommitments {
		if commitment.ExpiresAt > 0 && commitment.ExpiresAt <= nowHours {
			continue
		}
		active = append(active, commitment)
	}
	agent.NarrativeCommitments = active
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

// TransitionToFull reconstructs a plausible detailed state from accumulated
// events. The effects table, declared by the profile, maps each aggregated
// event kind to the consideration deltas it applies — the engine knows
// neither event nor consideration names. Events after hour and undeclared
// kinds are ignored; a nil table reconstructs nothing.
func TransitionToFull(agent *Agent, hour float64, effects []AggregatedEventEffect) {
	deltasByKind := make(map[string]map[string]float64, len(effects))
	for _, effect := range effects {
		deltasByKind[effect.Kind] = effect.Deltas
	}
	for _, event := range agent.AggregatedEvents {
		if event.AtHour > hour {
			continue
		}
		if deltas, declared := deltasByKind[event.Kind]; declared {
			applyAggregatedDeltas(agent, deltas)
		}
	}
	agent.AggregatedEvents = nil
	agent.SimulationLevel = Full
}

func applyAggregatedDeltas(agent *Agent, deltas map[string]float64) {
	for considerationID, delta := range deltas {
		consideration, exists := agent.Considerations[considerationID]
		if !exists {
			continue
		}
		consideration.Value = clamp(consideration.Value+delta, consideration.Min, consideration.Max)
		agent.Considerations[considerationID] = consideration
	}
}
