package main

import (
	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/internal/cli"
	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/pkg/build"
)

// import "github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/pkg/node"

var (
	BuildVersion string = ""
	BuildTime    string = ""
)

func main() {
	build.BuildVersion = BuildVersion
	build.BuildTime = BuildTime
	cli.Execute()
}
