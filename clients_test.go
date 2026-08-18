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
	"testing"

	"github.com/stretchr/testify/require"
)

// resetReconnectCallbacks clears the registered callbacks, restoring them when the test ends.
func resetReconnectCallbacks(t *testing.T) {
	t.Helper()

	reconnectCallbacksMu.Lock()
	existing := reconnectCallbacks
	reconnectCallbacks = nil
	reconnectCallbacksMu.Unlock()

	t.Cleanup(func() {
		reconnectCallbacksMu.Lock()
		reconnectCallbacks = existing
		reconnectCallbacksMu.Unlock()
	})
}

func TestOnClientActiveWithoutCallbacks(t *testing.T) {
	resetReconnectCallbacks(t)

	require.NotPanics(t, func() {
		onClientActive(context.Background(), "http://localhost:5051")
	})
}

func TestOnClientActiveCallsCallbacksWithAddress(t *testing.T) {
	resetReconnectCallbacks(t)

	addresses := make([]string, 0)
	addReconnectCallback(func(_ context.Context, address string) {
		addresses = append(addresses, address)
	})
	addReconnectCallback(func(_ context.Context, address string) {
		addresses = append(addresses, address)
	})

	onClientActive(context.Background(), "http://localhost:5051")

	require.Equal(t, []string{"http://localhost:5051", "http://localhost:5051"}, addresses)
}

func TestOnClientActiveCallsCallbacksOnEachActivation(t *testing.T) {
	resetReconnectCallbacks(t)

	calls := 0
	addReconnectCallback(func(_ context.Context, _ string) {
		calls++
	})

	onClientActive(context.Background(), "http://localhost:5051")
	onClientActive(context.Background(), "http://localhost:5052")

	require.Equal(t, 2, calls)
}

// TestAddReconnectCallbackDuringDispatch confirms that a callback registered while callbacks
// are being dispatched does not deadlock, as clients can become active at any time.
func TestAddReconnectCallbackDuringDispatch(t *testing.T) {
	resetReconnectCallbacks(t)

	addReconnectCallback(func(_ context.Context, _ string) {
		addReconnectCallback(func(_ context.Context, _ string) {})
	})

	require.NotPanics(t, func() {
		onClientActive(context.Background(), "http://localhost:5051")
	})

	reconnectCallbacksMu.RLock()
	defer reconnectCallbacksMu.RUnlock()
	require.Len(t, reconnectCallbacks, 2)
}
