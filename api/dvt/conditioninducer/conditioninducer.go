// Package conditioninducer implements the
// com.apple.instruments.server.services.ConditionInducer DVT channel.
// It allows simulating degraded network conditions and thermal states on the device.
package conditioninducer

import (
	"fmt"

	"github.com/larryhou/ix/api/dvt/remotesvr"
)

const channelIdentifier = `com.apple.instruments.server.services.ConditionInducer`

// Well-known condition group identifiers.
const (
	ConditionGroupNetwork = `com.apple.instruments.conditioninducer.network`
	ConditionGroupThermal = `com.apple.instruments.conditioninducer.thermal`
)

// Common network profile identifiers.
const (
	ProfileFullSpeed             = `FullSpeed`
	Profile100PercentLoss        = `100PercentLoss`
	ProfileSlowNetwork3GGood     = `SlowNetwork3GGoodReliability`
	ProfileSlowNetwork3GModerate = `SlowNetwork3GModerateReliability`
	ProfileSlowNetwork3GPoor     = `SlowNetwork3GPoorReliability`
	ProfileSlowNetworkEdgeGood   = `SlowNetworkEdgeGoodReliability`
	ProfileSlowNetworkEdgeModerate = `SlowNetworkEdgeModerateReliability`
	ProfileSlowNetworkEdgePoor   = `SlowNetworkEdgePoorReliability`
	ProfileHighLatency2G         = `HighLatency2G`
	ProfileVeryHighLatency2G     = `VeryHighLatency2G`
)

// Profile describes a single condition profile returned by AvailableConditions.
type Profile struct {
	Identifier  string
	Description string
}

// ConditionGroup describes a group of related simulation profiles.
type ConditionGroup struct {
	Identifier string
	Profiles   []*Profile
}

func New(svr *remotesvr.Service) (*Service, error) {
	id, err := svr.OpenChannel(channelIdentifier)
	if err != nil {
		return nil, err
	}
	return &Service{ch: svr.GetChannel(id)}, nil
}

type Service struct {
	ch *remotesvr.DTXChannel
}

// AvailableConditions returns all condition groups and their profiles supported by the device.
func (s *Service) AvailableConditions() ([]*ConditionGroup, error) {
	if err := s.ch.Send("availableConditionInducers", new(remotesvr.ArgumentAux), true); err != nil {
		return nil, err
	}

	obj, err := s.ch.Recv(nil)
	if err != nil {
		return nil, err
	}

	return decodeGroups(obj)
}

// Enable activates the condition profile identified by groupID and profileID.
// For example: Enable(ConditionGroupNetwork, ProfileSlowNetwork3GGood).
func (s *Service) Enable(groupID, profileID string) error {
	err := s.ch.Send(
		"enableConditionWithIdentifier:profileIdentifier:",
		new(remotesvr.ArgumentAux).Obj(groupID).Obj(profileID),
		true,
	)
	if err != nil {
		return err
	}
	// Consume ack (may be nil).
	_, err = s.ch.Recv(nil)
	return err
}

// Disable removes any active condition simulation. Fire-and-forget.
func (s *Service) Disable() error {
	return s.ch.Send("disableActiveCondition", new(remotesvr.ArgumentAux), false)
}

// decodeGroups parses the response from availableConditionInducers.
// Expected shape: []any of dicts, each with "identifier" (string) and
// "profiles" ([]any of dicts with "identifier" and "description").
func decodeGroups(raw any) ([]*ConditionGroup, error) {
	arr, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("conditioninducer: unexpected response type %T", raw)
	}

	groups := make([]*ConditionGroup, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		group := &ConditionGroup{}
		group.Identifier, _ = m["identifier"].(string)

		if profiles, ok := m["profiles"].([]any); ok {
			for _, pi := range profiles {
				pm, ok := pi.(map[string]any)
				if !ok {
					continue
				}
				p := &Profile{}
				p.Identifier, _ = pm["identifier"].(string)
				p.Description, _ = pm["description"].(string)
				group.Profiles = append(group.Profiles, p)
			}
		}
		groups = append(groups, group)
	}
	return groups, nil
}
