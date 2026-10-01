package dirk

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pkg/errors"
	e2wtypes "github.com/wealdtech/go-eth2-wallet-types/v2"
)

// participantsKey returns a canonical representation of an account's participant ID to endpoint map.
// Threshold recovery uses participant IDs as Lagrange coordinates, so accounts can only share a
// batched threshold request if their maps are identical.
func participantsKey(a *distributedAccount) string {
	ids := make([]uint64, 0, len(a.participants))
	for id := range a.participants {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	var sb strings.Builder
	for _, id := range ids {
		fmt.Fprintf(&sb, "%d=%s;", id, a.participants[id])
	}

	return sb.String()
}

// groupByParticipants returns the indices of accounts grouped by identical participant maps, in
// first-seen order.
func groupByParticipants(accounts []e2wtypes.Account) ([][]int, []string, error) {
	groups := make([][]int, 0, 1)
	keys := make([]string, 0, 1)
	positions := make(map[string]int)
	for i := range accounts {
		account, isAccount := accounts[i].(*distributedAccount)
		if !isAccount {
			return nil, nil, errors.New("account not of required type")
		}
		key := participantsKey(account)
		pos, exists := positions[key]
		if !exists {
			pos = len(groups)
			positions[key] = pos
			groups = append(groups, nil)
			keys = append(keys, key)
		}
		groups[pos] = append(groups[pos], i)
	}

	return groups, keys, nil
}

// needsParticipantSplit reports whether a batch must be split before threshold signing via a.
func needsParticipantSplit(a *distributedAccount, groups [][]int, keys []string) bool {
	if len(groups) > 1 {
		return true
	}

	return len(keys) == 1 && keys[0] != participantsKey(a)
}
