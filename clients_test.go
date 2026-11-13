// Copyright © 2026 Attestant Limited.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	nethttp "net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	httpclient "github.com/attestantio/go-eth2-client/http"
	"github.com/attestantio/vouch/services/metrics/null"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// TestFetchClientUpdatesProposalPreparationsWhenSynced confirms that a client built by
// fetchClient updates proposal preparations when it becomes synced, and not while it is only
// active and still syncing, when a beacon node can reject them.
func TestFetchClientUpdatesProposalPreparationsWhenSynced(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var syncing atomic.Bool
	syncing.Store(true)
	server := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		switch r.URL.Path {
		case "/eth/v1/node/version":
			_, _ = w.Write([]byte(`{"data":{"version":"test"}}`))
		case "/eth/v1/node/syncing":
			if syncing.Load() {
				_, _ = w.Write([]byte(`{"data":{"is_syncing":true,"is_optimistic":false,"el_offline":false,"head_slot":"100","sync_distance":"1000"}}`))
			} else {
				_, _ = w.Write([]byte(`{"data":{"is_syncing":false,"is_optimistic":false,"el_offline":false,"head_slot":"1100","sync_distance":"0"}}`))
			}
		default:
			w.WriteHeader(nethttp.StatusNotFound)
		}
	}))
	defer server.Close()
	viper.Set("eth2client.timeout", "2s")
	resetClientSyncedHandler(t)
	t.Cleanup(func() {
		viper.Reset()
		knownClientsMu.Lock()
		delete(knownClients, server.URL)
		knownClientsMu.Unlock()
	})

	preparer := newRecordingProposalPreparer()
	startProposalPreparationsUpdater(ctx, postBellatrixChainTime(12*time.Second), preparer)

	service, err := fetchClient(ctx, null.New(), server.URL)
	require.NoError(t, err)
	checker, isChecker := service.(interface{ CheckConnectionState(context.Context) })
	require.True(t, isChecker)

	// Active but still syncing.
	checker.CheckConnectionState(ctx)
	requireNoUpdate(t, preparer, 200*time.Millisecond)

	syncing.Store(false)
	checker.CheckConnectionState(ctx)
	requireUpdate(t, preparer, 2*time.Second)

	// Still synced, so no further transition.
	checker.CheckConnectionState(ctx)
	requireNoUpdate(t, preparer, 200*time.Millisecond)
}

// TestClientSyncedHookUsesServiceContext confirms that updates use the service context, not
// the context passed to the hook, which can belong to a request that has already completed.
func TestClientSyncedHookUsesServiceContext(t *testing.T) {
	server := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		switch r.URL.Path {
		case "/eth/v1/node/version":
			_, _ = w.Write([]byte(`{"data":{"version":"test"}}`))
		case "/eth/v1/node/syncing":
			_, _ = w.Write([]byte(`{"data":{"is_syncing":false,"is_optimistic":false,"el_offline":false,"head_slot":"1","sync_distance":"0"}}`))
		default:
			w.WriteHeader(nethttp.StatusNotFound)
		}
	}))
	defer server.Close()
	resetClientSyncedHandler(t)

	serviceCtx, cancelService := context.WithCancel(context.Background())
	defer cancelService()
	preparer := newRecordingProposalPreparer()
	startProposalPreparationsUpdater(serviceCtx, postBellatrixChainTime(12*time.Second), preparer)

	// Built without hooks, so that only the explicit call below reaches the handler.
	service, err := httpclient.New(context.Background(),
		httpclient.WithAddress(server.URL),
		httpclient.WithTimeout(2*time.Second),
		httpclient.WithLogLevel(zerolog.Disabled),
	)
	require.NoError(t, err)
	httpService, isHTTPService := service.(*httpclient.Service)
	require.True(t, isHTTPService)

	hookCtx, cancelHook := context.WithCancel(context.Background())
	cancelHook()
	clientHooks.OnSynced(hookCtx, httpService)

	update := requireUpdate(t, preparer, 2*time.Second)
	require.NoError(t, update.ctx.Err())
	cancelService()
	require.ErrorIs(t, update.ctx.Err(), context.Canceled)
}
