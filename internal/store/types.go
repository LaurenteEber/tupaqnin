package store

import (
	"encoding/json"
	"errors"
)

// Sentinel errors returned by the store (specification §2, backlog 0.4).
var (
	ErrVersionConflict = errors.New("store: version conflict")
	ErrNoChange        = errors.New("store: no change")
	ErrNotFound        = errors.New("store: not found")
	ErrInvalidType     = errors.New("store: invalid type")
	ErrInvalidState    = errors.New("store: state not allowed for type")
)

// ObjectType is a closed set (specification table 1.1, glossary §2).
type ObjectType string

const (
	ObjectRequest           ObjectType = "request"
	ObjectCriterion         ObjectType = "criterion"
	ObjectUnderstandingCase ObjectType = "understanding_case"
	ObjectPremise           ObjectType = "premise"
	ObjectSection           ObjectType = "section"
	ObjectDecision          ObjectType = "decision"
	ObjectFigure            ObjectType = "figure"
	ObjectDatum             ObjectType = "datum"
	ObjectCitation          ObjectType = "citation"
	ObjectSource            ObjectType = "source"
	ObjectOpenQuestion      ObjectType = "open_question"
	ObjectCard              ObjectType = "card"
	ObjectProposal          ObjectType = "proposal"
	ObjectSignoff           ObjectType = "signoff"
	ObjectAudienceQuestion  ObjectType = "audience_question"
	ObjectRound             ObjectType = "round"
)

// LabelPrefix is the short-label prefix for the type (glossary §8, T-2).
func (t ObjectType) LabelPrefix() string {
	switch t {
	case ObjectRequest:
		return "req"
	case ObjectCriterion:
		return "crit"
	case ObjectUnderstandingCase:
		return "case"
	case ObjectPremise:
		return "prem"
	case ObjectSection:
		return "sec"
	case ObjectDecision:
		return "dec"
	case ObjectFigure:
		return "fig"
	case ObjectDatum:
		return "dat"
	case ObjectCitation:
		return "cit"
	case ObjectSource:
		return "src"
	case ObjectOpenQuestion:
		return "oq"
	case ObjectCard:
		return "card"
	case ObjectProposal:
		return "prop"
	case ObjectSignoff:
		return "sign"
	case ObjectAudienceQuestion:
		return "aq"
	case ObjectRound:
		return "rnd"
	default:
		return ""
	}
}

// Valid reports whether t is one of the table 1.1 types.
func (t ObjectType) Valid() bool { return t.LabelPrefix() != "" }

// AllowsState reports whether state is admitted for this type
// (specification table 1.1). Types with no states admit only "".
// The computed section state "ready" is never stored (§1.1).
func (t ObjectType) AllowsState(state string) bool {
	var allowed []string
	switch t {
	case ObjectCriterion, ObjectPremise:
		allowed = []string{"current", "contradicted"}
		if t == ObjectCriterion {
			allowed = []string{"current"}
		}
	case ObjectUnderstandingCase:
		allowed = []string{"unconfirmed", "confirmed", "corrected"}
	case ObjectSection:
		allowed = []string{"draft", "signed", "changed_since_signoff", "stale"}
	case ObjectDecision:
		allowed = []string{"agent", "human", "confirmed", "in_open_question", "superseded", "revoked"}
	case ObjectOpenQuestion:
		allowed = []string{"open", "resolved"}
	case ObjectCard:
		allowed = []string{"open", "answered", "obsolete"}
	case ObjectProposal:
		allowed = []string{"open", "accepted", "discarded"}
	case ObjectAudienceQuestion:
		allowed = []string{"open", "addressed", "declined"}
	case ObjectRound:
		allowed = []string{"open", "closed"}
	case ObjectRequest, ObjectFigure, ObjectDatum, ObjectCitation, ObjectSource, ObjectSignoff:
		allowed = []string{""}
	default:
		return false
	}
	for _, s := range allowed {
		if state == s {
			return true
		}
	}
	return false
}

// EdgeType is a closed set (specification table 1.2, glossary §3).
type EdgeType string

const (
	EdgeCovers              EdgeType = "covers"
	EdgeUsedIn              EdgeType = "used_in"
	EdgeComesFrom           EdgeType = "comes_from"
	EdgeUses                EdgeType = "uses"
	EdgeApplies             EdgeType = "applies"
	EdgeDependsOnAssumption EdgeType = "depends_on_assumption"
	EdgeBlocks              EdgeType = "blocks"
	EdgeSupersedes          EdgeType = "supersedes"
	EdgeExplains            EdgeType = "explains"
	EdgeAbout               EdgeType = "about"
)

// Valid reports whether t is one of the table 1.2 edge types.
func (t EdgeType) Valid() bool {
	switch t {
	case EdgeCovers, EdgeUsedIn, EdgeComesFrom, EdgeUses, EdgeApplies,
		EdgeDependsOnAssumption, EdgeBlocks, EdgeSupersedes, EdgeExplains, EdgeAbout:
		return true
	default:
		return false
	}
}

// EventType is a closed set (specification table 1.3, glossary §4).
type EventType string

const (
	EventStateChanged             EventType = "state_changed"
	EventStepEnded                EventType = "step_ended"
	EventModeSet                  EventType = "mode_set"
	EventHumanMessage             EventType = "human_message"
	EventStopReport               EventType = "stop_report"
	EventDecisionSeen             EventType = "decision_seen"
	EventEdit                     EventType = "edit"
	EventEditUndone               EventType = "edit_undone"
	EventSuspiciousEditAlert      EventType = "suspicious_edit_alert"
	EventSignoffCase              EventType = "signoff_case"
	EventSignoffPaused            EventType = "signoff_paused"
	EventBriefSignoff             EventType = "brief_signoff"
	EventAudienceQuestionDeclined EventType = "audience_question_declined"
	EventUsageChange              EventType = "usage_change"
	EventSourceNewVersion         EventType = "source_new_version"
	EventRoleChanged              EventType = "role_changed"
	EventAudienceChanged          EventType = "audience_changed"
	EventEdge                     EventType = "edge"
	EventAnswerWithoutReason      EventType = "answer_without_reason"
	EventActTimed                 EventType = "act_timed"
	EventMarkedCritical           EventType = "marked_critical"
	EventProbe                    EventType = "probe"
	EventStartup                  EventType = "startup"
	EventOutputRetry              EventType = "output_retry"
)

// Valid reports whether t is one of the table 1.3 event types.
func (t EventType) Valid() bool {
	switch t {
	case EventStateChanged, EventStepEnded, EventModeSet, EventHumanMessage,
		EventStopReport, EventDecisionSeen, EventEdit, EventEditUndone,
		EventSuspiciousEditAlert, EventSignoffCase, EventSignoffPaused,
		EventBriefSignoff, EventAudienceQuestionDeclined, EventUsageChange,
		EventSourceNewVersion, EventRoleChanged, EventAudienceChanged,
		EventEdge, EventAnswerWithoutReason, EventActTimed, EventMarkedCritical,
		EventProbe, EventStartup, EventOutputRetry:
		return true
	default:
		return false
	}
}

// Object is a graph node with its current version body (§1.1).
type Object struct {
	ProjectID      string          `json:"project_id"`
	ID             string          `json:"id"`
	Label          string          `json:"label"`
	Type           ObjectType      `json:"type"`
	State          string          `json:"state"`
	CurrentVersion int             `json:"current_version"`
	Body           json.RawMessage `json:"body"`
}

// Edge is a graph edge (§1.2). Only the current version has Current set.
type Edge struct {
	ProjectID string   `json:"project_id"`
	ID        string   `json:"id"`
	Type      EdgeType `json:"type"`
	FromID    string   `json:"from_id"`
	ToID      string   `json:"to_id"`
	Anchor    string   `json:"anchor"`
	Version   int      `json:"version"`
	Current   bool     `json:"current"`
}

// EventInput is what a caller appends to the history (table 1.3).
// Seq, ProjectID and CreatedAt are assigned by the store.
type EventInput struct {
	Type           EventType
	Actor          string
	AgentRole      string
	ModelAndEffort string
	Reason         string
	ObjectID       string
	BaseVersion    int
	NewVersion     int
	TurnID         string
	Data           any
}

// Event is one append-only history row (§1.3, §8 of bloque-1-estados).
type Event struct {
	ProjectID      string          `json:"project_id"`
	Seq            int             `json:"seq"`
	Type           EventType       `json:"type"`
	Actor          string          `json:"actor"`
	AgentRole      string          `json:"agent_role"`
	ModelAndEffort string          `json:"model_and_effort"`
	Reason         string          `json:"reason"`
	ObjectID       string          `json:"object_id"`
	BaseVersion    int             `json:"base_version"`
	NewVersion     int             `json:"new_version"`
	TurnID         string          `json:"turn_id"`
	Data           json.RawMessage `json:"data"`
	CreatedAt      string          `json:"created_at"`
}
