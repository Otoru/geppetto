// Package geppetto is the stateless, deterministic Utility AI core used by
// geppetto. It turns an agent's considerations and the actions advertised by
// nearby providers into a selected action. The package owns scoring,
// eligibility, stochastic selection, simulation ticks, preemption, and the
// deterministic reconciliation of contested batch decisions. It does not
// discover world entities, open transports, persist NPC state, or apply the
// actual resource debit performed by a game.
//
// Games normally construct a Decider with WithProfile or WithTuning and pass
// an explicit seed to Decide, Rank, or DecideBatch. Timed simulation is a
// separate mutable-agent lifecycle: construct a Simulator with the same
// options and pass an explicit seed to its Advance method. The separation
// keeps a point-in-time decision from accidentally owning a game's NPC state,
// while making both deterministic paths equally available in-process.
//
// The architecture is intentionally an inversion of the usual scripted-NPC
// relationship. An agent does not contain a table such as "if hungry, find a
// refrigerator". Objects, places, events, and other actors advertise what
// they offer through AdvertisedAction values. The agent scores those promises
// against its current considerations. A new world object therefore adds an
// advertisement rather than a new branch in every NPC's code: the content
// surface grows approximately with N+M instead of requiring N*M agent/object
// integrations.
//
// # Considerations and their updaters
//
// A Consideration is a bounded scalar signal such as HUNGER, ENERGY, THREAT,
// AMMO, COVER, or a game-specific mission value. The engine treats all of
// these uniformly. The updater, rather than the consideration name, explains
// how a value changes:
//
//   - LinearDecay subtracts rate*deltaHours, and LinearRegen adds it.
//   - EventDriven values are left untouched by a tick; the game changes them
//     through explicit events or action effects.
//   - PerceptionDriven, RelationshipDriven, and ContextAggregate values read
//     the corresponding World map, or call their configured Value function.
//     If a map has no entry for the consideration, its previous value is
//     retained rather than silently reset to zero.
//
// Every result is clamped to the consideration's own Min and Max. A missing
// or invalid range (Max <= Min) makes its response pressure zero, so profiles
// must declare bounds explicitly. Trait consideration deltas are additive rate
// adjustments and affect only the linear updater kinds. ApplyGradualDeltas
// delivers an action's promised deltas at a constant rate over its estimated
// duration, also clamping after each application; a delta for a consideration
// the agent does not have is ignored.
//
// Response curves turn a raw value into pressure before scoring. Convex curves
// use a clamped normalized distance from the maximum raised to an exponent;
// Linear curves use that normalized distance directly; Step curves switch
// between Below and Above at Threshold; Logistic curves transition around a
// Midpoint. A convex exponent of zero inherits the profile default (normally
// 2), while a configured exponent is an intentional per-consideration
// override. An unknown curve kind evaluates to zero rather than creating an
// implicit rule.
//
// DefaultTuning supplies the reference calibration: convex exponent 2,
// intrinsic-priority weight 5, distance reference 10 world units, selection
// TopK 3, temperature 1, reconciliation TopK 5, preemption margin 1.5,
// convention-break probability 0, perception noise 0, a FULL tick of one
// minute of game time, and a SIMPLIFIED tick of one hour. For parameters that
// use zero as "not configured", zero falls back to these defaults; it is not a
// way to disable priority or stochastic selection. In particular, a profile
// cannot request a true zero temperature through this fallback mechanism.
// Profiles can still choose genre-specific values: social and open-world
// profiles use higher temperatures and preemption margins for emergent,
// stubborn behavior; tactical profiles use lower values and rely more on
// perception noise; survival profiles give stronger weights to hunger and
// thirst.
//
// The value zero in action capacity is not the same kind of zero as provider
// capacity. An AdvertisedAction with Capacity == 0 has no action-level limit;
// a provider with Capacity == 0 has no available provider slots. The action
// meaning is deliberate: proto3 encodes an omitted numeric field as zero, and
// older clients omit action_capacities. Treating that zero as blocking would
// make every pre-existing action unusable after the field was added. The
// provider meaning remains blocking. An agent must have a free slot at both
// levels, and the stricter level wins.
//
// # Utility scoring
//
// Candidates first pass binary eligibility gates. The provider must have a
// free slot, the action must not be saturated, the provider must be within the
// action's inclusive advertisement radius, all preconditions must hold, the
// cost must be payable, and no active narrative commitment may contradict the
// action tags. Failed gates remove a candidate; they do not give it a lower
// score.
//
// For an eligible action x and agent A, the implementation computes:
//
//	benefit = sum(pressure(c) * expectedGain(c, x))
//	score = (benefit + intrinsicPriority(x)*default priority weight)
//	        * M_personality * M_context * M_distance
//	        - weightedCost
//
// The expected gain is saturated per consideration:
//
//	expectedGain(c, x) = min(delta(x,c), c.Max-c.Value)
//
// This cap prevents an oversized promise from dominating when an agent has
// little left to gain. It is intentionally asymmetric: it limits positive
// gains only. A negative delta remains a real cost to the consideration,
// because Max-Value is non-negative and cannot clip that negative promise.
// This is how mixed actions, such as an enjoyable but tiring activity, remain
// meaningful without a special case.
//
// Each factor has a distinct design job. Pressure makes urgency nonlinear, so
// a critical need can outweigh a low-value comfort without a hard-coded mode
// switch. Intrinsic priority lets content authors express a structural reason
// to prefer an action, but it is added before the other multipliers and thus
// still obeys distance. Personality multiplies tag modifiers and the domain
// preference, allowing traits to amplify or suppress a whole promise without
// making an action attractive when it helps none of the agent's
// considerations. Distance uses the hyperbolic multiplier
// 1/(1+d/default distance reference), making a nearby modest opportunity compete with
// a distant ideal one. Cost is subtracted last so positive personality,
// context, or distance modifiers cannot make a resource bill disappear.
//
// Context has an important, deliberate asymmetry in this implementation. For
// every active Context, the tag multiplier is applied twice:
//
//	M_context = product(contextTagMultiplier * contextTagMultiplier)
//
// Thus a nominal context multiplier of 2 for an action tag contributes 4 to
// the score. Applying the multiplier only once lets the effect dissolve in
// the later softmax and does not produce the intended doubling of observable
// action frequency. Do not simplify this to a single factor without
// recalibrating the behavior and its acceptance expectations.
//
// weightedCost is also more specific than the name might suggest. The real
// implementation is:
//
//	sum(cost / max(1, available/cost))
//
// For an eligible positive cost, that is cost^2/available. Doubling the cost
// therefore quadruples its penalty, while halving the available stock doubles
// the penalty for the same cost. Eligibility checks affordability first; the
// score then models scarcity. The penalty is not multiplied by the positive
// factors above.
//
// # Selection and deliberate imperfection
//
// Candidates are stable-sorted by descending utility. SelectAction keeps the
// configured SelectionTopK (default 3) and samples with a numerically stable
// softmax. The maximum utility is subtracted before exponentiation, which
// preserves the distribution while avoiding overflow. A non-positive
// SelectionTemperature falls back to the default 1.0; temperature zero is
// therefore not an accidental way to request argmax. Lower temperatures make
// choices more competent and predictable, while higher temperatures flatten
// the differences. A TopK of one degenerates to argmax.
//
// Imperfection is a design principle, not an implementation defect. Always
// taking the maximum makes NPCs predictable, exploitable, and narratively
// sterile. The stochastic pick is retained as the first preference in a batch
// so that contention resolution does not silently turn the system back into
// argmax. Commitment to a current action, limited advertisement radius, and
// personality or context biases provide further, tunable sources of behavior
// that is imperfect but still legible. PerceptionNoise adds zero-mean Gaussian
// noise with the configured standard deviation to each perception-driven
// consideration in a temporary scoring copy; the observed world state remains
// unchanged. ConventionBreakProbability defines the normal convention as the
// highest-utility eligible action. When it succeeds, the normal softmax pick
// is replaced by a uniformly selected eligible action with strictly lower
// utility, when one exists. The aim is variety that an observer can explain,
// not minimum error.
//
// # Ticks, preemption, and simulation detail
//
// Advance is the in-process entry point. Its TickRequest carries the absolute
// simulation hour, world, desired simulation level, and optional simplified
// event; TickResult reports the effective level and simulated duration. The
// integrating simulation chooses the target level because only it knows
// visibility and importance. FullTickHours and SimplifiedTickHours determine
// the duration returned for their respective levels, falling back to the named
// engine defaults when unset.
//
// Advance first expires commitments whose ExpiresAt is at or before NowHours.
// A simplified advance records its optional event at NowHours. Returning to
// Full applies the profile's aggregated-event effects before the detailed
// interval. Tick then updates considerations, applies gradual deltas to the
// current action, and clears a completed action. An idle agent dequeues the
// next FIFO ActionQueue entry, or makes an autonomous selection when no work
// is queued; a continuing action is then considered for preemption. A newly
// started action therefore does not receive its first gradual delta until the
// following interval. The engine owns queue progression through EnqueueAction,
// while the integrating game remains responsible for enqueuing intent and
// debiting costs. Preemption abandons the current action rather than requeueing
// partial work; pending FIFO entries remain available after the replacement
// action finishes.
//
// Preemption requires at least one consideration below its configured
// CriticalThreshold. A player-queued action is protected unless some value
// reaches imminent collapse, read from that consideration's own
// CriticalThreshold; a consideration without one falls back to the engine's
// default imminent-collapse threshold. The replacement
// is selected through the same stochastic pipeline, and it must strictly
// exceed preemptionMargin times the current action's frozen
// ContinuationUtility. Equality does not preempt. This frozen utility is the
// snapshot captured when the action was selected, so an action that was very
// valuable when started can be deliberately difficult to interrupt.
//
// ApplyAggregatedEvent records coarse events while an agent is simplified;
// TransitionToFull applies the profile's AggregatedEventEffects table to the
// events that occurred by the target hour, clamping each resulting
// consideration value, then discards the event log and returns the agent to
// Full. The table is data: the engine knows neither event nor consideration
// names, and a profile without it reconstructs nothing. The mechanism is
// intentionally small and stateless: the game remains responsible for
// deciding which distant events to aggregate.
//
// # Batch contention and determinism
//
// RankedPreferences performs the normal stochastic selection first, then
// retains descending-utility fallbacks up to ReconciliationTopK (default 5).
// ResolveContention consumes those lists in rounds. Every open agent claims
// its current candidate; claims are grouped by provider, and each provider's
// contenders are ordered by increasing physical distance, then agent ID, then
// input index. Provider slots are Capacity minus the reported Occupants count.
// Action slots are tracked per provider instance and start at action Capacity
// minus action Occupancy; zero action capacity means unlimited. A candidate
// is granted only if both levels have a slot.
//
// Proximity, not utility, decides a dispute. This is a physical rule: the
// agent that can reach the resource first gets it. Hunger does not accelerate
// anyone, so a hungry distant agent must not teleport priority away from a
// nearby agent merely because its score is higher. A loser advances through
// its own preference list. That can displace a provisional winner elsewhere,
// so rounds continue until one complete round has no displacement. An agent
// that exhausts its list receives no ActionInstance. Because every cursor only
// moves forward through a finite retained list, termination is guaranteed.
//
// # Stability
//
// Within a single version of this module, the same input and the same seed
// always produce the same result. That guarantee is tested, and it is the
// whole guarantee. Across versions there is no determinism commitment: the
// scoring formula and its calibration may change in any minor release, and
// the same seed may then select a different action. A game that needs
// deterministic replay or save compatibility must pin the module version.
//
// The scoring phase is independent and can run in parallel; the reconciliation
// phase is a small serial reduction over bounded lists. Map iteration order and
// worker completion order cannot change the result: each agent appears in one
// claim list per round, and the within-provider order is explicit. With the
// same input and random seed, the selected preferences and final assignments
// are deterministic. No contention state survives the call.
package geppetto
