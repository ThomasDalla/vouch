package dirk

import (
	"testing"

	"github.com/stretchr/testify/require"
	e2wtypes "github.com/wealdtech/go-eth2-wallet-types/v2"
)

func TestGroupByParticipants(t *testing.T) {
	ovh, ald, tux := NewEndpoint("ovh", 8882), NewEndpoint("alderlake", 8882), NewEndpoint("tuxedo", 8882)
	orderA := map[uint64]*Endpoint{1: ovh, 2: ald, 3: tux}
	orderB := map[uint64]*Endpoint{1: ald, 2: tux, 3: ovh}
	a1 := &distributedAccount{participants: orderA}
	b1 := &distributedAccount{participants: orderB}
	a2 := &distributedAccount{participants: map[uint64]*Endpoint{3: NewEndpoint("tuxedo", 8882), 1: NewEndpoint("ovh", 8882), 2: NewEndpoint("alderlake", 8882)}}
	b2 := &distributedAccount{participants: orderB}

	groups, keys, err := groupByParticipants([]e2wtypes.Account{a1, b1, a2, b2})
	require.NoError(t, err)
	require.Equal(t, [][]int{{0, 2}, {1, 3}}, groups)
	require.True(t, needsParticipantSplit(a1, groups, keys))

	groups, keys, err = groupByParticipants([]e2wtypes.Account{b1, b2})
	require.NoError(t, err)
	require.Equal(t, [][]int{{0, 1}}, groups)
	require.False(t, needsParticipantSplit(b1, groups, keys))
	require.True(t, needsParticipantSplit(a1, groups, keys))
}
