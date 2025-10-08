package wire

import "testing"

func TestEnvelopeRoundTrip(t *testing.T) {
	env := Envelope{ID: NewRPCID(), Type: "msg", Payload: []byte("hi")}
	raw := env.Marshal()
	out, err := Unmarshal(raw)
	t.Logf("Envelope after round-trip: %+v", out)
	if err != nil {
		t.Fatal(err)
	}
	if out.Type != env.Type || string(out.Payload) != "hi" || out.ID == (RPCID{}) {
		t.Fatalf("bad roundtrip: %+v", out)
	}
}

func TestNewRPCID_Unique(t *testing.T) {
	const N = 200_000
	seen := make(map[[20]byte]struct{}, N)
	for i := 0; i < N; i++ {
		id := NewRPCID()
		var key [20]byte
		copy(key[:], id[:])
		if _, ok := seen[key]; ok {
			t.Fatalf("duplicate at %d", i)
		}
		seen[key] = struct{}{}
	}
}
