// kademlia/node/value_integration_test.go
package node

// import (
// 	"context"
// 	"testing"
// 	"time"
// )

// func TestStoreAndFindValue_Value(t *testing.T) {
// 	nA, _ := NewNode("127.0.0.1:0", "")
// 	nA.Start()
// 	defer nA.Close()
// 	nB, _ := NewNode("127.0.0.1:0", "")
// 	nB.Start()
// 	defer nB.Close()

// 	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
// 	defer cancel()
// 	key, err := nA.StoreValue(ctx, nB.Svc.Addr(), "hello")
// 	if err != nil {
// 		t.Fatalf("StoreValue: %v", err)
// 	}

// 	v, contacts, err := nA.FindValue(ctx, nB.Svc.Addr(), key)
// 	if err != nil || v == nil || *v != "hello" || contacts != nil {
// 		t.Fatalf("FindValue value branch failed: v=%v contacts=%v err=%v", v, contacts, err)
// 	}
// }
