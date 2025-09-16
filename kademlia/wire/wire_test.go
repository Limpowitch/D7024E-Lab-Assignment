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
