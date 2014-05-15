package main

import (
	"encoding/xml"
	"io/ioutil"
	"strings"
	"os"
)

var cmdLogLevels = &Command{
	Run:   runLogLevels,
	Usage: "loglevels",
	Short: "change debug log levels",
	Long: ` 
	Change Log levels for execute anonymous and running tests

	Examples:
		force loglevels show => show the current log levels 
		force loglevels ALL:INFO => set all categories to info
		force loglevels APEX_CODE:ERROR APEX_PROFILING:INFO => set each category specifically 
	`,
}

type DebugOptions struct {
	Category string `xml:"category"`
	Level    string `xml:"level"`
}

type DebuggingHeader struct {
	XMLName    xml.Name       `xml:"DebuggingHeader"`
	Categories []DebugOptions `xml:"categories"`
}

func runLogLevels(cmd *Command, args []string) {
	//first check if the debugHeader.xml exists and if not create it regardless, so it's there in the future 
	if _, err := os.Stat("debugHeader.xml"); os.IsNotExist(err) {
			def := "<apex:DebuggingHeader><apex:categories><apex:category>ALL</apex:category><apex:level>INFO</apex:level></apex:categories></apex:DebuggingHeader>"
    		res := []byte(def)
    		ioutil.WriteFile("debugHeader.xml", res, 0755)
	}
	if len(args) < 1 {
		ErrorAndExit("Must specify options")
	}else if strings.ToUpper(args[0]) == "SHOW" {
		debug_header := &DebuggingHeader{}
		file, _ := ioutil.ReadFile("debugHeader.xml")
		if err := xml.Unmarshal([]byte(file), debug_header); err != nil {
			ErrorAndExit("Error reading debug file")
		}
		for _,element := range debug_header.Categories {
			println(element.Category + ":" + element.Level)
		}
	}else{
		categories_available := map[string]bool{
			"ALL":            true,
			"WORKFLOW":       true,
			"VALIDATION":     true,
			"CALLOUT":        true,
			"APEX_CODE":      true,
			"APEX_PROFILING": true,
		}

		levels_available := map[string]bool{
			"ERROR":  true,
			"WARN":   true,
			"INFO":   true,
			"DEBUG":  true,
			"FINE":   true,
			"FINER":  true,
			"FINEST": true,
		}
		debug_header := &DebuggingHeader{}
		var all_val, soap string
		file, _ := ioutil.ReadFile("debugHeader.xml")
		if err := xml.Unmarshal([]byte(file), debug_header); err != nil {
			return
		}
		new_opts := make(map[string]string)
		for _, element := range args {
			opt_pairs := strings.Split(string(element), ":")
			if strings.ToUpper(opt_pairs[0]) == "ALL" {
				all_val = strings.ToUpper(opt_pairs[1])
				break
			} else {
				new_opts[strings.ToUpper(opt_pairs[0])] = strings.ToUpper(opt_pairs[1])
			}
		}
		if all_val != "" {
			soap = "<apex:DebuggingHeader><apex:categories><apex:category>ALL</apex:category><apex:level>" + all_val + "</apex:level></apex:categories></apex:DebuggingHeader>"
		} else {
			for index, _ := range debug_header.Categories {
				if debug_header.Categories[index].Category == "ALL" {
					//debug_header.Categories[index] = nil
					debug_header.Categories = debug_header.Categories[:index+copy(debug_header.Categories[index:], debug_header.Categories[index+1:])]
					continue
				}
				level, ex := new_opts[debug_header.Categories[index].Category]
				_, cat_valid := categories_available[debug_header.Categories[index].Category]
				_, lvl_valid := levels_available[debug_header.Categories[index].Level]
				if ex && cat_valid && lvl_valid {
					debug_header.Categories[index].Level = level
					delete(new_opts, string(debug_header.Categories[index].Category))
				}
			}
			if len(new_opts) != 0 {
				for key, _ := range new_opts {
					debug_header.Categories = append(debug_header.Categories, DebugOptions{Category: key, Level: new_opts[key]})
				}
			}
			//Constructing xml this way because go doesn't currently support marshaling xml with namespace using colon
			soap = "<apex:DebuggingHeader>"
			for _, element := range debug_header.Categories {
				soap += "<apex:categories><apex:category>" + element.Category + "</apex:category><apex:level>" + element.Level + "</apex:level></apex:categories>"
			}
			soap += "</apex:DebuggingHeader>"
		}
		res := []byte(soap)
		ioutil.WriteFile("debugHeader.xml", res, 0755)
	}
}
