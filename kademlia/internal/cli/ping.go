package cli

import (
	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/cmd/node"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(PingCmd)
}

var PingCmd = &cobra.Command{
	Use:   "Ping",
	Short: "Ping a Node",
	Long:  "Ping a Node",
	Run: func(cmd *cobra.Command, args []string) {
		node, _ := node.NewNode("localhost") // localhost is a temporary thing, should be replaced with an input of the users choice
		node.PingNode()                      // PingNode is not yet implimented
	},
}

// var TalkCmd = &cobra.Command{
// 	Use:   "talk",
// 	Short: "Say something",
// 	Long:  "Say something",
// 	Run: func(cmd *cobra.Command, args []string) {
// 		hellworld := helloworld.NewHelloWorld()
// 		hellworld.Talk()
// 	},
// }
