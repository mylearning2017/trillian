// Copyright 2017 Google Inc. All Rights Reserved.
//
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

package client

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/google/trillian"
	"github.com/google/trillian/testonly"
	"github.com/google/trillian/testonly/integration"
)

const timeout = 100 * time.Millisecond

func TestAddGetLeaf(t *testing.T) {
	// TODO: Build a GetLeaf method and test a full get/set cycle.
}

// addSequencedLeaves is a temporary stand-in function for tests until the real API gets built.
func addSequencedLeaves(env *integration.LogEnv, client VerifyingLogClient, leaves [][]byte) error {
	// TODO(gdbelvin): Replace with batch API.
	// TODO(gdbelvin): Replace with AddSequencedLeaves API.
	for _, l := range leaves {
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(timeout))
		defer cancel()
		if err, want := client.AddLeaf(ctx, l), codes.DeadlineExceeded; grpc.Code(err) != want {
			return fmt.Errorf("AddLeaf(%v): %v, want, %v", l, err, want)
		}
		env.Sequencer.OperationLoop() // Sequence the new leaves in-order.
	}
	return nil
}

func TestGetByIndex(t *testing.T) {
	env, err := integration.NewLogEnv(context.Background(), 0, "TestGetByIndex")
	if err != nil {
		t.Fatal(err)
	}
	defer env.Close()
	logID, err := env.CreateLog()
	if err != nil {
		t.Fatalf("Failed to create log: %v", err)
	}

	cli := trillian.NewTrillianLogClient(env.ClientConn)
	client := New(logID, cli, testonly.Hasher, env.PublicKey)
	// Add a few test leaves.
	leafData := [][]byte{
		[]byte("A"),
		[]byte("B"),
		[]byte("C"),
	}

	if err := addSequencedLeaves(env, client, leafData); err != nil {
		t.Errorf("Failed to add leaves: %v", err)
	}

	for i, l := range leafData {
		leaf, err := client.GetByIndex(context.Background(), int64(i))
		if err != nil {
			t.Errorf("Failed to GetByIndex(%v): %v", i, err)
		}
		if got, want := leaf.LeafValue, l; !bytes.Equal(got, want) {
			t.Errorf("GetByIndex(%v) = %x, want %x", i, got, want)
		}
	}
}

func TestListByIndex(t *testing.T) {
	env, err := integration.NewLogEnv(context.Background(), 0, "TestGetByIndex")
	if err != nil {
		t.Fatal(err)
	}
	defer env.Close()
	logID, err := env.CreateLog()
	if err != nil {
		t.Fatalf("Failed to create log: %v", err)
	}

	cli := trillian.NewTrillianLogClient(env.ClientConn)
	client := New(logID, cli, testonly.Hasher, env.PublicKey)
	// Add a few test leaves.
	leafData := [][]byte{
		[]byte("A"),
		[]byte("B"),
		[]byte("C"),
	}

	if err := addSequencedLeaves(env, client, leafData); err != nil {
		t.Errorf("Failed to add leaves: %v", err)
	}

	// Fetch leaves.
	leaves, err := client.ListByIndex(context.Background(), 0, 3)
	if err != nil {
		t.Errorf("Failed to ListByIndex: %v", err)
	}
	for i, l := range leaves {
		if got, want := l.LeafValue, leafData[i]; !bytes.Equal(got, want) {
			t.Errorf("ListIndex()[%v] = %v, want %v", i, got, want)
		}
	}
}

func TestAddLeaf(t *testing.T) {
	env, err := integration.NewLogEnv(context.Background(), 0, "TestAddLeaf")
	if err != nil {
		t.Fatal(err)
	}
	defer env.Close()
	logID, err := env.CreateLog()
	if err != nil {
		t.Fatalf("Failed to create log: %v", err)
	}

	cli := trillian.NewTrillianLogClient(env.ClientConn)
	for _, test := range []struct {
		desc    string
		client  trillian.TrillianLogClient
		wantErr bool
	}{
		{
			desc:   "success 1",
			client: &MockLogClient{c: cli},
		},
		{
			desc:   "success 2",
			client: &MockLogClient{c: cli},
		},
		{
			desc:    "invalid inclusion proof",
			client:  &MockLogClient{c: cli, mGetInclusionProof: true},
			wantErr: true,
		},
		{
			desc:    "invalid consistency proof",
			client:  &MockLogClient{c: cli, mGetConsistencyProof: true},
			wantErr: true,
		},
	} {
		client := New(logID, test.client, testonly.Hasher, env.PublicKey)
		{
			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(timeout))
			defer cancel()
			if err, want := client.AddLeaf(ctx, []byte(test.desc)), codes.DeadlineExceeded; grpc.Code(err) != want {
				t.Errorf("AddLeaf(%v): %v, want, %v", test.desc, err, want)
				continue
			}
		}
		env.Sequencer.OperationLoop() // Sequence the new node.
		err := client.AddLeaf(context.Background(), []byte(test.desc))
		if got := err != nil; got != test.wantErr {
			t.Errorf("AddLeaf(%v): %v, want error: %v", test.desc, err, test.wantErr)
		}
	}
}

func TestUpdateRoot(t *testing.T) {
	env, err := integration.NewLogEnv(context.Background(), 0, "TestUpdateRoot")
	if err != nil {
		t.Fatal(err)
	}
	defer env.Close()
	logID, err := env.CreateLog()
	if err != nil {
		t.Fatalf("Failed to create log: %v", err)
	}
	cli := trillian.NewTrillianLogClient(env.ClientConn)
	client := New(logID, cli, testonly.Hasher, env.PublicKey)

	before := client.Root().TreeSize

	{
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(timeout))
		defer cancel()
		if err, want := client.AddLeaf(ctx, []byte("foo")), codes.DeadlineExceeded; grpc.Code(err) != want {
			t.Errorf("AddLeaf(): %v, want, %v", err, want)
		}
	}

	env.Sequencer.OperationLoop() // Sequence the new node.
	if err := client.UpdateRoot(context.Background()); err != nil {
		t.Error(err)
	}
	if got, want := client.Root().TreeSize, before; got <= want {
		t.Errorf("Tree size after add Leaf: %v, want > %v", got, want)
	}
}
