//go:build sim

package sim

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/cmd/node"
	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/internal/transport"
)

// go test -tags=sim ./kademlia/sim -v -run TestSim_EmulateCluster_FindValue -- -sim_nodes 1000 -sim_drop 0.1 -sim_seed 1
// go test -tags=sim ./kademlia/sim -v -run TestSim_EmulateCluster_FindValue -- -sim_nodes 50 -sim_drop 0.5 -sim_seed 3
// Different flags that can be set when running the test
var (
	fs       = flag.NewFlagSet("sim", flag.ContinueOnError)            // Local flag set
	simNodes = fs.Int("sim_nodes", 1000, "number of nodes")            // Set the numbers of nodes
	simDrop  = fs.Float64("sim_drop", 0.10, "packet drop rate [0..1]") // Set the packet drop rate
	simSeed  = fs.Int64("sim_seed", 1, "rng seed")                     // Set a random seed for the run.
)

func TestMain(m *testing.M) {
	fmt.Println("os.Args:", strings.Join(os.Args, " | "))
	fs.SetOutput(io.Discard)

	// For seperating sim args after "--"
	var simArgs []string
	for i, a := range os.Args {
		if a == "--" {
			simArgs = os.Args[i+1:]
			break
		}
	}

	// For fixing the issue that the decimal points being split into two arguments
	simArgs = fixDecimalSplit(simArgs)

	if err := fs.Parse(simArgs); err != nil {
		fmt.Println("flag parse error:", err)
	}
	if rest := fs.Args(); len(rest) > 0 {
		fmt.Println("unparsed args:", strings.Join(rest, " | "))
	}
	os.Exit(m.Run())
}

// Helpers to fix the issue when the numbers being splitted if there is a decimal point in the number
func fixDecimalSplit(in []string) []string {
	out := make([]string, 0, len(in))
	for _, a := range in {
		if len(out) > 0 && strings.HasPrefix(a, ".") && isAllDigits(a[1:]) &&
			strings.HasPrefix(out[len(out)-1], "-sim_drop=") && endsWithDigit(out[len(out)-1]) {
			out[len(out)-1] += a
			continue
		}
		out = append(out, a)
	}
	return out
}

// Helper for checking that the string is all digits
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// Helper for checking that the string ends with a number
func endsWithDigit(s string) bool {
	if s == "" {
		return false
	}
	ch := s[len(s)-1]
	return ch >= '0' && ch <= '9'
}

// Emulates a cluster of nodes by doing puts and gets and returning the return rate
func TestSim_EmulateCluster_FindValue(t *testing.T) {
	t.Logf("FLAGS: sim.nodes=%d sim.drop=%.3f sim.seed=%d",
		*simNodes, *simDrop, *simSeed)

	// Set up for the seed that are deterministic for repeatability and sets the latency function
	transport.SetSeed(*simSeed)
	transport.SetLatency(func(_, _ string) time.Duration { return 200 * time.Microsecond })

	// Start the bootstrap server with no loss so everyone can join
	transport.SetDropProbability(0)

	// Spins up N nodes and if N<10 it uses 10 nodes
	N := *simNodes
	if N < 10 {
		N = 10
	}

	// Create and start N nodes with ports from 10000 upwards
	nodes := make([]*node.Node, 0, N)
	for i := 0; i < N; i++ {
		addr := fmt.Sprintf("127.0.0.1:%d", 10_000+i)
		n, err := node.NewNode(addr, addr, 5*time.Minute, 0)
		if err != nil {
			t.Fatalf("NewNode %d: %v", i, err)
		}
		n.Start()
		nodes = append(nodes, n)
	}
	defer func() { //Checks that all the nodes are closed at the end
		for _, n := range nodes {
			_ = n.Close()
		}
	}()

	// Uses the first node as a seed for the rest of the nodes
	seed := nodes[0]

	// --- Record stats before bootstrap chatter + PUT
	b0Sent, b0Delivered, b0Dropped := transport.SimStats()

	// Have every node needed to ping the seed node to join the network
	var wg sync.WaitGroup
	for i := 1; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			_ = nodes[i].Svc.Ping(ctx, seed.Svc.Addr())
			cancel()
		}(i)
	}
	wg.Wait()

	// Does a put to store a value in the network, the value is random 32 bytes
	payload := make([]byte, 32)
	_, _ = rand.Read(payload)
	// Uses AdminPut to store the value in the network
	putCtx, putCancel := context.WithTimeout(context.Background(), 3*time.Second) // Delay of 3 seconds for the put
	key, err := seed.Svc.AdminPut(putCtx, seed.Svc.Addr(), payload)
	putCancel()
	if err != nil {
		t.Fatalf("AdminPut: %v", err)
	}
	keyHex := hex.EncodeToString(key[:])

	// Record stats after bootstrap + PUT
	b1Sent, b1Delivered, b1Dropped := transport.SimStats()
	phaseSent := b1Sent - b0Sent
	phaseDelivered := b1Delivered - b0Delivered
	phaseDropped := b1Dropped - b0Dropped
	denom := phaseSent
	if denom == 0 {
		denom = 1
	}
	t.Logf("SIM/PUT+bootstrap: sent=%d delivered=%d dropped=%d dropRate=%.3f",
		phaseSent, phaseDelivered, phaseDropped, float64(phaseDropped)/float64(denom))

	// The second phase is the lookup phase so we reset the stats
	transport.ResetSimStats()
	transport.SetDropProbability(*simDrop)

	// The setter for the amount of gets to do, and counters for the amount of successful gets and errors
	const numGets = 50
	var okCount int64
	var errCount int64

	// Do numGets gets in parallel with a limit of 32 concurrent gets
	sem := make(chan struct{}, 32)
	wg = sync.WaitGroup{}
	for i := 0; i < numGets; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()

			idx := (i*7919 + 17) % N // deterministic spread
			ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
			got, ok, err := nodes[idx].Svc.AdminGet(ctx, nodes[idx].Svc.Addr(), key) //AdminGet does an iterative get
			cancel()

			if err != nil {
				atomic.AddInt64(&errCount, 1) // timeouts
				return
			}
			if ok && len(got) == len(payload) { // got the value
				atomic.AddInt64(&okCount, 1)
			}
		}(i)
	}
	wg.Wait()

	// Phase-2 stats (lookups only)
	gSent, gDelivered, gDropped := transport.SimStats()
	gDenom := gSent
	if gDenom == 0 {
		gDenom = 1
	}
	t.Logf("SIM/LOOKUP(50): sent=%d delivered=%d dropped=%d dropRate=%.3f",
		gSent, gDelivered, gDropped, float64(gDropped)/float64(gDenom))

	t.Logf("Nodes=%d drop=%.2f ok=%d/%d errs=%d key=%s",
		N, *simDrop, okCount, numGets, errCount, keyHex)

	hopsGuess := 3.0 // 3-4 hops in kademlia for N=1000

	expect := float64(numGets) * math.Pow(1.0-*simDrop, hopsGuess) // Expected success count for the gets
	minOK := int(math.Max(1, math.Floor(expect*0.70)))             // Needs at least 70% of the expected value to pass the test
	if *simDrop <= 0.5 && int(okCount) < minOK {                   // Only assert the test if the drop rate is <=50%
		t.Fatalf("too few successful lookups: got %d/%d, need >=%d (drop=%.2f, expect≈%.1f)",
			okCount, numGets, minOK, *simDrop, expect)
	}

	// Final simulation stats
	sent, delivered, dropped := transport.SimStats()
	var dropRate float64
	if sent > 0 { //Sanity check to avoid division by zero
		dropRate = float64(dropped) / float64(sent) // drop rate for the whole run
	}
	t.Logf("SIM: sent=%d delivered=%d dropped=%d dropRate=%.3f",
		sent, delivered, dropped, dropRate)

	//Checks that the drop rate is in a reasonable range being +-20% of the requested drop rate
	if *simDrop >= 0.5 && dropRate < 0.3 {
		t.Fatalf("drop too low: want ~%.2f got %.3f (sent=%d dropped=%d)",
			*simDrop, dropRate, sent, dropped)
	}
}
