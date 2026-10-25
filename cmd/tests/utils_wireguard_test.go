package tests

import (
	"testing"
	"scutum/cmd/internal/utils"
)

// fixedOutputRunner returns a fixed byte slice for every Output call.
type fixedOutputRunner struct{ out []byte }

func (f *fixedOutputRunner) Run(_ string, _ ...string) error                      { return nil }
func (f *fixedOutputRunner) Output(_ string, _ ...string) ([]byte, error)         { return f.out, nil }
func (f *fixedOutputRunner) CombinedOutput(_ string, _ ...string) ([]byte, error) { return f.out, nil }
func (f *fixedOutputRunner) StdinPipe(_ string, _ ...string) (utils.PipeWriter, error) {
	return &mockPipe{}, nil
}

func TestGetPeerEndpoint_ParsesOutput(t *testing.T) {
	key := "abc123pubkey="
	for _, c := range []struct {
		name    string
		output  string
		want    string
		wantErr bool
	}{
		{
			name:   "known peer with endpoint",
			output: key + "\t203.0.113.10:51820\nother=\t1.2.3.4:51820\n",
			want:   "203.0.113.10:51820",
		},
		{
			name:   "known peer with no endpoint yet",
			output: key + "\t(none)\n",
			want:   "",
		},
		{
			name:    "peer not in output",
			output:  "other=\t1.2.3.4:51820\n",
			wantErr: true,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			utils.SetCommandRunner(&fixedOutputRunner{out: []byte(c.output)})
			ep, err := utils.GetPeerEndpoint("wg0", key)
			if c.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ep != c.want {
				t.Errorf("GetPeerEndpoint = %q, want %q", ep, c.want)
			}
		})
	}
}

func TestWireGuardUtils(t *testing.T) {
	t.Run("DerivePublicKey", func(t *testing.T) {
		// Valid 32-byte base64 encoded private key
		priv := "GAs/80L9fL8fL8fL8fL8fL8fL8fL8fL8fL8fL8fL8f8=" 
		pub, err := utils.DerivePublicKey(priv)
		if err != nil {
			t.Fatalf("DerivePublicKey failed: %v", err)
		}
		if pub == "" {
			t.Error("expected non-empty public key")
		}
	})

	t.Run("DerivePublicKey-Invalid", func(t *testing.T) {
		_, err := utils.DerivePublicKey("too-short")
		if err == nil {
			t.Error("expected error for invalid key")
		}
	})
}
