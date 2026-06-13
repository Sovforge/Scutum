package tests

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"scutum/cmd/internal/store"
)

func TestNodeGroups_CreateAndList(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if err := s.CreateNodeGroup(ctx, "g1", "prod-eu", "EU production nodes"); err != nil {
		t.Fatalf("CreateNodeGroup: %v", err)
	}

	groups, err := s.ListNodeGroups(ctx)
	if err != nil {
		t.Fatalf("ListNodeGroups: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].Name != "prod-eu" {
		t.Errorf("Name = %q, want prod-eu", groups[0].Name)
	}
}

func TestNodeGroups_AddAndRemoveMember(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Create a node directly in the DB so we can add it to a group
	nodeID := uuid.New().String()
	if err := s.CreateNode(ctx, store.NodeRecord{ID: nodeID, Name: "test-node", Type: "remote", Address: "10.0.0.1/32", PublicKey: "pubkey"}); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	s.CreateNodeGroup(ctx, "g1", "my-group", "")

	if err := s.AddNodeToGroup(ctx, "g1", nodeID); err != nil {
		t.Fatalf("AddNodeToGroup: %v", err)
	}

	nodes, err := s.ListNodesInGroup(ctx, "g1")
	if err != nil {
		t.Fatalf("ListNodesInGroup: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node in group, got %d", len(nodes))
	}

	if err := s.RemoveNodeFromGroup(ctx, "g1", nodeID); err != nil {
		t.Fatalf("RemoveNodeFromGroup: %v", err)
	}

	nodes, _ = s.ListNodesInGroup(ctx, "g1")
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes after removal, got %d", len(nodes))
	}
}

func TestNodeLabels_SetAndGet(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	nodeID := uuid.New().String()
	s.CreateNode(ctx, store.NodeRecord{ID: nodeID, Name: "test-node", Type: "remote", Address: "10.0.0.2/32", PublicKey: "pubkey2"})

	labels := map[string]string{"env": "prod", "region": "eu-west"}
	if err := s.SetNodeLabels(ctx, nodeID, labels); err != nil {
		t.Fatalf("SetNodeLabels: %v", err)
	}

	got, err := s.GetNodeLabels(ctx, nodeID)
	if err != nil {
		t.Fatalf("GetNodeLabels: %v", err)
	}
	if got["env"] != "prod" {
		t.Errorf("env = %q, want prod", got["env"])
	}
	if got["region"] != "eu-west" {
		t.Errorf("region = %q, want eu-west", got["region"])
	}
}
