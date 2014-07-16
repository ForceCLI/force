package main

import (
	"os"
	//"encoding/json"
	"fmt"
	"strings"
	//"io/ioutil"
	//"path/filepath"
)

var cmdAura = &Command{
	Usage: "aura",
	Short: "force aura push -e=<filepath>",
	Long: `
	force aura push -e=<fullFilePath>

	force aura create -t=<entity type> <entityName>

	force aura delete -e=<fullFilePath>

	force aura list

	`,
}

func init() {
	cmdAura.Run = runAura
}

var (
	auraentity = cmdAura.Flag.String("e", "", "fully qualified file name for entity")
	auraentitytype = cmdAura.Flag.String("t", "", "aura entity type")
	auraentityname = cmdAura.Flag.String("n", "", "aura entity name")
)

func runAura(cmd *Command, args []string) {
	if err := cmd.Flag.Parse(args[1:]); err != nil {
		os.Exit(2)
	}
	force, _ := ActiveForce()

	subcommand := args[0];

	switch strings.ToLower(subcommand) {
	case "create":
		if *auraentitytype == "" || *auraentityname == "" {
			fmt.Println("Must specify entity type and name")
			os.Exit(2)
		}

	case "delete":
	case "list":
		bundles, err := force.GetAuraBundlesList()
		if err != nil {
			ErrorAndExit("Ooops")
		}
		for _, bundle := range bundles.Records {
			fmt.Println(bundle["DeveloperName"])
		}
	case "push":
		runPushAura(cmd, args)
	}
	fmt.Println(subcommand)
	fmt.Println(*auraentity)
}

