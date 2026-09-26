package geppetto

// UpdateConsiderations advances every updater, applies trait rate adjustments,
// and clamps each result to its configured bounds.
func UpdateConsiderations(agent *Agent, world World, deltaHours float64) {
	for considerationID, consideration := range agent.Considerations {
		adjustedRate := updaterRate(agent.Personality.Traits, considerationID, consideration.Updater.Rate)

		switch consideration.Updater.Kind {
		case LinearDecay:
			consideration.Value -= adjustedRate * deltaHours
		case LinearRegen:
			consideration.Value += adjustedRate * deltaHours
		case PerceptionDriven:
			updateWorldDrivenConsideration(agent, world, world.Perception, considerationID, &consideration)
		case RelationshipDriven:
			updateWorldDrivenConsideration(agent, world, world.Relationships, considerationID, &consideration)
		case ContextAggregate:
			updateWorldDrivenConsideration(agent, world, world.ContextValues, considerationID, &consideration)
		case EventDriven:
			// Events mutate this consideration explicitly, so a tick leaves it unchanged.
		}

		consideration.Value = clamp(consideration.Value, consideration.Min, consideration.Max)
		agent.Considerations[considerationID] = consideration
	}
}

func updaterRate(traits []Trait, considerationID string, baseRate float64) float64 {
	adjustedRate := baseRate
	for _, trait := range traits {
		adjustedRate += trait.ConsiderationDelta[considerationID]
	}
	return adjustedRate
}

func updateWorldDrivenConsideration(agent *Agent, world World, values map[string]float64, considerationID string, consideration *Consideration) {
	if consideration.Updater.Value != nil {
		consideration.Value = consideration.Updater.Value(*agent, world)
		return
	}

	if worldValue, ok := values[considerationID]; ok {
		consideration.Value = worldValue
	}
}

// ApplyGradualDeltas applies an action's promised deltas at a constant rate.
func ApplyGradualDeltas(agent *Agent, instance *ActionInstance, deltaHours float64) {
	if instance == nil || instance.Action.EstimatedDuration <= 0 {
		return
	}

	for considerationID, delta := range instance.Action.Deltas {
		consideration, exists := agent.Considerations[considerationID]
		if !exists {
			continue
		}

		consideration.Value = clamp(
			consideration.Value+delta*deltaHours/instance.Action.EstimatedDuration,
			consideration.Min,
			consideration.Max,
		)
		agent.Considerations[considerationID] = consideration
	}
	instance.Elapsed += deltaHours
}
