package geppetto

import "math/rand/v2"

// Option configures a Decider or Simulator. Options are immutable values and
// may be reused when constructing multiple SDK objects.
type Option func(*options)

type options struct {
	profile   Profile
	tuning    Tuning
	hasTuning bool
}

// WithProfile supplies the profile used for tuning and aggregated-event
// reconstruction. WithTuning overrides only the profile's tuning values.
func WithProfile(profile Profile) Option {
	return func(options *options) { options.profile = profile }
}

// WithTuning supplies tuning without requiring a profile. When combined with
// WithProfile, this option overrides Profile.Tuning regardless of option order.
func WithTuning(tuning Tuning) Option {
	return func(options *options) {
		options.tuning = tuning
		options.hasTuning = true
	}
}

func newOptions(configurers []Option) options {
	configured := options{tuning: DefaultTuning()}
	for _, configure := range configurers {
		if configure != nil {
			configure(&configured)
		}
	}
	if !configured.hasTuning {
		configured.tuning = configured.profile.Tuning
	}
	return configured
}

// Decider selects actions from a supplied agent and provider snapshot. It has
// no mutable simulation state; pass an explicit seed to every operation so
// independent decisions remain reproducible.
type Decider struct{ tuning Tuning }

// NewDecider constructs a decision API. WithProfile supplies its tuning, and
// WithTuning is useful when a caller owns model data without a JSON profile.
func NewDecider(options ...Option) Decider {
	configured := newOptions(options)
	return Decider{tuning: configured.tuning}
}

// Decide selects an action for one agent from the supplied provider snapshot.
// The seed owns the complete pseudo-random stream for this decision.
func (decider Decider) Decide(agent Agent, providers []AffordanceProvider, seed uint64) *ActionInstance {
	rng := rand.New(rand.NewPCG(seed, seed+1))
	return SelectAction(agent, providers, decider.tuning, rng)
}

// Rank produces one agent's ordered, contention-ready preferences. The seed
// owns the complete pseudo-random stream for this ranking.
func (decider Decider) Rank(agent Agent, providers []AffordanceProvider, seed uint64) []Candidate {
	rng := rand.New(rand.NewPCG(seed, seed+1))
	return RankedPreferences(agent, providers, decider.tuning, rng)
}

// Reconcile assigns contested provider and action slots from rankings returned
// by Rank. The operation is deterministic and consumes no random values.
func (decider Decider) Reconcile(agents []Agent, preferences [][]Candidate) []*ActionInstance {
	return ResolveContention(agents, preferences)
}

// DecideBatch ranks each agent with seed plus its stable slice index, then
// reconciles contested capacity. It is a convenience for callers that do not
// need their own parallel ranking phase.
func (decider Decider) DecideBatch(agents []Agent, providers []AffordanceProvider, seed uint64) []*ActionInstance {
	preferences := make([][]Candidate, len(agents))
	for index, agent := range agents {
		preferences[index] = decider.Rank(agent, providers, seed+uint64(index))
	}
	return decider.Reconcile(agents, preferences)
}

// Simulator advances mutable agent state through the timed simulation loop.
// It retains the profile's aggregation table but no agent state or random
// state, so callers can safely share it across independent agents.
type Simulator struct{ profile Profile }

// NewSimulator constructs a timed simulation API. WithProfile provides both
// event reconstruction data and tuning; WithTuning overrides its tuning.
func NewSimulator(options ...Option) Simulator {
	configured := newOptions(options)
	configured.profile.Tuning = configured.tuning
	return Simulator{profile: configured.profile}
}

// Advance advances agent through one complete timed simulation step. The seed
// owns the random stream for this step, including any selection or preemption.
func (simulator Simulator) Advance(agent *Agent, providers []AffordanceProvider, request TickRequest, seed uint64) TickResult {
	rng := rand.New(rand.NewPCG(seed, seed+1))
	return Advance(agent, simulator.profile, providers, request, rng)
}
