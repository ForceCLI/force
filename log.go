package main

import (
	"fmt"
	"strings"
	"io/ioutil"
	"regexp"
	"strconv"
)

type DebugNodeInterface interface {
	printCSV( stack string) string
	getChilds() []DebugNodeInterface 
	match( args []string ) bool
	getStackName() string
}
type DebugNode struct {
// EVENT TYPE	
	EventType string	
// TIME	DATA
	StartTime uint64
	EndTime uint64
	ElapsedTime uint64
// TREE NAVIGATION
	Parent * DebugNode
	Childs [] DebugNodeInterface
//
	Rows uint16
}

type CodeNode struct {
	DebugNode
	Line string
	CodeUnit string
	ClassId string
	Level uint16
}

type DMLNode struct {
	DebugNode
	Line string	
	Operation string
	Type string	
}

type SOQLNode struct {
	DebugNode
	Line string
	Aggregations uint16
	SOQL string
}

type HeapNode struct {
	DebugNode
	Line string
	Bytes uint32
}

var cmdLog = &Command{
	Run:   getLog,
	Usage: "log",
	Short: "Fetch and Parse debug logs",
	Long: `
Fetch and Parse debug logs

Examples:

  force log [list] [format] [ <field>:<value> ]

  force log 07Le000000sKUylEAG [format] [<field>:<value>]

  force log <<file>> [format] [<field>:<value>]
 
 * formats: csv, json, text
 * fields: 
 	- EventType: SOQL, DML, CODE, HEAP
 	- 
`,
}


func init() {
}

func (n DebugNode) getChilds() []DebugNodeInterface {
	return n.Childs 
}
func (n DebugNode) match( args []string ) bool {
	var condition bool
	for _, value := range args {
		options := strings.Split(value, ":")
		if len(options) != 2 {
			ErrorAndExit(fmt.Sprintf("Missing value for trace flag %s", value))
		}
		switch ( strings.ToLower(options[0]) )  {
			case "eventtype":
				condition = n.EventType == options[1] 
			default:
				ErrorAndExit("Format %s not supported\n\n", options[0])		
		}
		if condition == false {
			return false
		}
	}
	
	return true
}
func (n DebugNode) getStackName() string {
	return ""
}

func (n CodeNode) getStackName() string {
	return n.CodeUnit + "\n"
}

func (n DebugNode) printCSV(stack string) string {
	return fmt.Sprintf( "\"%v\",%v,%v,%v,%v", stack, n.EventType, n.StartTime, n.EndTime, n.ElapsedTime ) 
}

func (n CodeNode) printCSV(stack string) string {
	return fmt.Sprintf( "\"%v\",\"%v\",%v,%v,%v,\"%v\"", stack, n.CodeUnit, n.StartTime, n.EndTime, n.ElapsedTime, n.Line ) 
}

func (n HeapNode) printCSV(stack string) string {
	// StartTime,NumBytes,Line
	return fmt.Sprintf( "\"%v\",%v,%v,\"%v\"", stack, n.StartTime, n.Bytes, n.Line ) 
}

func (n SOQLNode) printCSV(stack string) string {
	return fmt.Sprintf( "\"%v\",\"%v\",%v,%v,%v,%v,%v,\"%v\"", stack, n.SOQL, n.StartTime, n.EndTime, n.ElapsedTime, n.Rows, n.Aggregations , n.Line ) 
}

func (n DMLNode) printCSV(stack string) string {
	return fmt.Sprintf( "\"%v\",\"%v\",\"%v\",%v,%v,%v,%v,\"%v\"",stack,  n.Operation, n.Type, n.StartTime, n.EndTime, n.ElapsedTime, n.Rows, n.Line ) 
}

func displayCSV ( current DebugNodeInterface, stack string, args []string ) {
	for _, node := range current.getChilds() {
		if node.match( args )  {
			fmt.Println( node.printCSV(stack) ) 
		}
		displayCSV( node, stack + node.getStackName(), args )
	}
}

func parseLog ( log string) (debugTree DebugNode) {
	var timestamp uint64
	var rowData []string
	// var err error
	root := DebugNode{"ROOT", 0, 0, 0, nil, []DebugNodeInterface{} , 0}
   	current := &root
	rows := strings.Split(string(log), "\n")[1:]
	re := regexp.MustCompile("[0-9]+")

	for _, row := range rows {
		rowData = strings.Split(string(row), "|")
		if ( len(rowData) <= 1 ) {
			continue
		}

        timestampf, err := strconv.ParseFloat( re.FindString( strings.Split(rowData[0], " ")[1] ) , 10 )
		if err != nil {
			ErrorAndExit( row + ":\n" + err.Error())
		}
        timestamp = uint64(timestampf)

       	var eventType = rowData[1]
        switch( eventType  ) {        	
        	case "EXECUTION_STARTED":
        		root.StartTime = uint64(timestamp)
       		
        	case "CODE_UNIT_STARTED", "METHOD_ENTRY", "SYSTEM_METHOD_ENTRY":
			// 17:01:49.056 (56251266)|CODE_UNIT_STARTED|[EXTERNAL]|06612000001qh7a|VF: /apex/FieloPRM_VFP_Page
        		var newCode CodeNode
        		if len(rowData) == 5  {
        			newCode = CodeNode { DebugNode{"CODE", uint64(timestamp), 0, 0, current, []DebugNodeInterface{}, 0 }, "", rowData[4], rowData[3] }
    			} else {
        			newCode = CodeNode { DebugNode{"CODE", uint64(timestamp), 0, 0, current, []DebugNodeInterface{}, 0 }, "", rowData[3], "" }
    			}
        		current.Childs = append( current.Childs, &newCode )
        		current = &newCode.DebugNode
        	case "SOQL_EXECUTE_BEGIN": 
        	// 17:01:50.421 (1421361727)|SOQL_EXECUTE_BEGIN|[98]|Aggregations:0|SELECT Name, FieloEE__ExternalName__c FROM Menu__c WHERE Id = :tmpVar1
        		aggregations, err := strconv.ParseFloat(re.FindString(rowData[3]), 10 )
				if err != nil {
					ErrorAndExit( row + ":\n" + err.Error())
				}        		
        		newSOQL  := SOQLNode { DebugNode{"SOQL", uint64(timestamp), 0, 0, current, nil , 0}, rowData[2], uint16(aggregations), rowData[4] }
        		current.Childs = append( current.Childs, &newSOQL )
        		current = &newSOQL.DebugNode

        	case "HEAP_ALLOCATE":
				// 14:32:33.434 (17434285698)|HEAP_ALLOCATE|[72]|Bytes:3
        		bytes, err := strconv.ParseFloat(re.FindString(rowData[3]), 10)
				if err != nil {
					ErrorAndExit( row + ":\n" + err.Error())
				}        		
        		newHeap  := HeapNode { DebugNode{"HEAP", uint64(timestamp), 0, 0, current, nil , 0}, rowData[2], uint32(bytes) }
        		current.Childs = append( current.Childs, &newHeap )
        		//current = &newHeap.DebugNode				

        	case "DML_BEGIN": 
        	// 17:01:50.444 (1444795717)|DML_BEGIN|[139]|Op:Upsert|Type:FieloEE__FrontEndSessionData__c|Rows:1
        		numRows , err := strconv.ParseFloat(re.FindString(rowData[5]), 10)
				if err != nil {
					ErrorAndExit( row + ":\n" + err.Error())
				}        		
        		newDML  := DMLNode { DebugNode{"DML", uint64(timestamp), 0, 0, current, nil, uint16(numRows)}, rowData[2] ,rowData[3], rowData[4] }
				current.Childs = append( current.Childs, &newDML)
        		current = &newDML.DebugNode

        	case "EXECUTION_FINISHED", "CODE_UNIT_FINISHED", "METHOD_EXIT", "SYSTEM_METHOD_EXIT", "SOQL_EXECUTE_END", "DML_END":        		
        		current.EndTime = uint64(timestamp)
        		current.ElapsedTime = current.EndTime - current.StartTime
        		// Add rows
        		if ( eventType == "SOQL_EXECUTE_END" ) {
        			numRows, err := strconv.ParseFloat(re.FindString(rowData[3]), 10)
        			current.Rows = uint16(numRows)
					if err != nil {
						ErrorAndExit( row + ":\n" + err.Error())
					}					
        		}
	    		current = current.Parent
	    
        	case "LIMIT_USAGE_FOR_NS":

        	default:
        	   continue;
        }      

	}

	return root
}
func getLog(cmd *Command, args []string) {
	var format = "text"
	if len(args) >= 2  {
		format = args[1]
	}
	force, _ := ActiveForce()
	if len(args) == 0 || args[0] == "list" {
		records, err := force.QueryLogs()
		if err != nil {
			ErrorAndExit(err.Error())
		}
		DisplayForceRecordsf(records.Records, format)		
	} else {
		var log string
		var file []byte
		var err error
		if args[0][0:3] == "07L"  {
			log, err = force.RetrieveLog(args[0])
		} else {			
			file, err = ioutil.ReadFile(args[0])			
			log = string(file)
		}
		if err != nil {
			ErrorAndExit(err.Error())
		}

		root := parseLog ( log )

		switch format {
			case "csv":
				displayCSV( root, "", args[2:] )
				
			case "json":

			case "json-pretty":

			case "text":
				fmt.Println(log)
			default:
				fmt.Printf("Format %s not supported\n\n", format)
		}

	}
}
