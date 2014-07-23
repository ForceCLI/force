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
	Short: "force aura push -fileName=<filepath>",
	Long: `
	force aura push -f=<fullFilePath>

	force aura create -t=<entity type> <entityName>

	force aura delete -f=<fullFilePath>

	force aura list

	`,
}

func init() {
	cmdAura.Run = runAura
	//cmdAura.Flag.StringVar(fileName, "fileName", "", "fully qualified file name for entity")
	cmdAura.Flag.StringVar(auraentitytype, "entitytype", "", "fully qualified file name for entity")
	cmdAura.Flag.StringVar(auraentityname, "entityname", "", "fully qualified file name for entity")
}

var (
	//fileName     = cmdAura.Flag.String("f", "", "fully qualified file name for entity")
	auraentitytype = cmdAura.Flag.String("t", "", "aura entity type")
	auraentityname = cmdAura.Flag.String("n", "", "aura entity name")
)

func runAura(cmd *Command, args []string) {
	if err := cmd.Flag.Parse(args[1:]); err != nil {
		os.Exit(2)
	}
	force, _ := ActiveForce()

	subcommand := args[0]

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
}
