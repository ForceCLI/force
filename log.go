package main

import (
	"fmt"
	"strings"
	"io/ioutil"
	"encoding/json"
	"regexp"
	"strconv"
)

type DebugNodeInterface interface {
	printNode( stack string) string
	printMeasures() string
	match( args []string ) bool
	getStackName() string
	getNode() DebugNode
	addChild( DebugNodeInterface )
	setEndTime( uint64, uint32  )
	addChildTime( uint64 )
}


type DebugBaseNode struct {
// EVENT TYPE	
	EventType string	
// TIME	DATA
	StartTime uint64
	EndTime uint64
	ElapsedTime uint64
	ChildElapseTime uint64	
// FILE POSITION
	StartRow uint32
	EndRow uint32
// NAVIGATION
	Childs [] DebugNodeInterface
}

// Need to Split in two due to json recursion :) 
type DebugNode struct {
	DebugBaseNode
// TREE NAVIGATION
	Parent DebugNodeInterface
}

type RootNode struct {
	DebugNode
	SQOLRows uint16
	SQOLTime uint64
	SQOLNum uint16
	DMLRows uint16
	DMLTime uint64
	DMLNum uint16
	ExecTime uint64
	HEAPSize uint32
}

type CodeNode struct {
	DebugNode
	SubType string
	Line string
	CodeUnit string	
	ClassId string
	IdleTime uint64
	Level int
}

type DMLNode struct {
	DebugNode
	Line string	
	Operation string
	Type string
	Rows uint16		
}

type SOQLNode struct {
	DebugNode
	Line string
	Aggregations uint16
	SOQL string
	Rows uint16	
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

  force log [download] [ <field>:<value> ] 

  force log [delete] [ <field>:<value> ] 

  force log [parse] [format] <<file>> [<field>:<value>]

  force log <<DebugLog ID>> [format] [<field>:<value>]
  
 
 * formats: csv, json, text
 * fields: 
 	- EventType: SOQL, DML, CODE, HEAP
 	- 
`,
}


func init() {
}

func (n * CodeNode) match( args []string ) bool {
	var condition bool
	for _, value := range args {
		options := strings.Split(value, ":")
		if len(options) != 2 {
			ErrorAndExit(fmt.Sprintf("Missing value for trace flag %s", value))
		}
		switch ( strings.ToLower(options[0]) )  {
			case "eventtype":
				condition = n.EventType == options[1] 
			case "elapsedtime":
		        elapsedtime, err := strconv.ParseInt( options[1] , 10, 64 )
				if err != nil {
					ErrorAndExit( "Level Must be a Number")
				}
				condition = n.ElapsedTime >= uint64(elapsedtime)
			case "level":
		        level, err := strconv.ParseInt( options[1] , 10, 16 )
				if err != nil {
					ErrorAndExit( "Level Must be a Number")
				}
				condition = n.Level == int(level)
			default:
				ErrorAndExit("Format %s not supported\n\n", options[0])		
		}
		if condition == false {
			return false
		}
	}	
	return true
}

func (n * DebugNode) match( args []string ) bool {
	var condition bool
	for _, value := range args {
		options := strings.Split(value, ":")
		if len(options) != 2 {
			ErrorAndExit(fmt.Sprintf("Missing value for trace flag %s", value))
		}
		switch ( strings.ToLower(options[0]) )  {
			case "eventtype":
				condition = n.EventType == options[1] 
			case "elapsedtime":
		        elapsedtime, err := strconv.ParseInt( options[1] , 10, 64 )
				if err != nil {
					ErrorAndExit( "Level Must be a Number")
				}
				condition = n.ElapsedTime >= uint64(elapsedtime)
			default:
				ErrorAndExit("Format %s not supported\n\n", options[0])		
		}
		if condition == false {
			return false
		}
	}
	
	return true
}

func (n * DebugNode) addChildTime( elapsedtime uint64  )  {
	n.ChildElapseTime += elapsedtime	
}

func (n * DebugNode) setEndTime( endTime uint64 , endRow uint32 )  {
	n.EndRow = endRow
	n.EndTime = endTime
	n.ElapsedTime = endTime - n.StartTime	
	if ( n.Parent != nil ) {
		n.Parent.addChildTime( n.ElapsedTime	)
	}	
}

func (n * DebugNode) addChild( c DebugNodeInterface )  {
	n.Childs = append( n.Childs, c )
}
func (n * SOQLNode) addChild( c DebugNodeInterface )  {
	n.DebugNode.Childs = append( n.Childs, c )
}

func (n DebugNode) getNode() DebugNode {
	return n
}

func (n * DebugNode) getStackName() string {
	return ""
}

func (n * CodeNode) getStackName() string {
	return n.CodeUnit + ":" + n.Line + "\n"
}

func (n * DebugNode) printMeasures() string {
	reference := fmt.Sprintf("Rows=%v-%v", n.StartRow, n.EndRow )
	return fmt.Sprintf( "\"%v\",\"%v\",%v", n.EventType, reference, "Time",  n.ElapsedTime / 1000000 ) 
}

func (n * CodeNode) printMeasures() string {
	reference := fmt.Sprintf("Rows=%v-%v", n.StartRow, n.EndRow )
	return fmt.Sprintf( "\"%v\",\"%v\",\"%v\",%v", n.EventType, reference, "Time",  n.ElapsedTime / 1000000 ) + "\n" +
		fmt.Sprintf( "\"%v\",\"%v\",\"%v\",%v", n.EventType, reference, "IdleTime",  n.IdleTime  / 1000000 ) 
}

func (n * HeapNode) printMeasures() string {
	reference := fmt.Sprintf("Rows=%v-%v", n.StartRow, n.EndRow )
	return fmt.Sprintf( "\"%v\",\"%v\",\"%v\",%v", n.EventType, reference, "Memory",  n.Bytes ) 
}

func (n * SOQLNode) printMeasures() string {
	reference := fmt.Sprintf("Rows=%v-%v", n.StartRow, n.EndRow )
	return fmt.Sprintf( "\"%v\",\"%v\",\"%v\",%v", n.EventType, reference, "Time",  n.ElapsedTime  / 1000000) + "\n" +
		fmt.Sprintf( "\"%v\",\"%v\",\"%v\",%v", n.EventType, reference, "Aggregations",  n.Aggregations ) + "\n" +
		fmt.Sprintf( "\"%v\",\"%v\",\"%v\",%v", n.EventType, reference, "Rows",  n.Rows ) 
}

func (n * DMLNode) printMeasures() string {
	reference := fmt.Sprintf("Rows=%v-%v", n.StartRow, n.EndRow )
	return fmt.Sprintf( "\"%v\",\"%v\",\"%v\",%v", n.EventType, reference , "Time",  n.ElapsedTime / 1000000 ) + "\n" +
		fmt.Sprintf( "\"%v\",\"%v\",\"%v\",%v", n.EventType, reference, "Rows",  n.Rows ) 
}

func (n * DebugNode) printNode(stack string) string {
	return fmt.Sprintf( "\"%v\",%v,%v,%v,%v", stack, n.EventType, n.StartTime, n.EndTime, n.ElapsedTime ) 
}

func (n * RootNode) printNode(stack string) string {
	return fmt.Sprintf( "\"%v\",%v,%v,%v,%v,%v,%v,%v,%v,%v", n.EventType, n.SQOLRows, n.SQOLTime, n.SQOLNum, n.DMLRows, n.DMLTime, n.DMLNum, n.ExecTime, n.HEAPSize, n.ElapsedTime ) 
}

func (n * CodeNode) printNode(stack string) string {
	return fmt.Sprintf( "\"%v\",\"%v\",%v,%v,%v,%v,\"%v\",%v", stack, n.CodeUnit, n.StartTime, n.EndTime, n.ElapsedTime, n.IdleTime, n.Line, n.Level ) 
}

func (n * HeapNode) printNode(stack string) string {
	// StartTime,NumBytes,Line
	return fmt.Sprintf( "\"%v\",%v,%v,\"%v\"", stack, n.StartTime, n.Bytes, n.Line ) 
}

func (n * SOQLNode) printNode(stack string) string {
	return fmt.Sprintf( "\"%v\",\"%v\",%v,%v,%v,%v,%v,\"%v\"", stack, n.SOQL, n.StartTime, n.EndTime, n.ElapsedTime, n.Rows, n.Aggregations , n.Line ) 
}

func (n * DMLNode) printNode(stack string) string {
	return fmt.Sprintf( "\"%v\",\"%v\",\"%v\",%v,%v,%v,%v,\"%v\"",stack,  n.Operation, n.Type, n.StartTime, n.EndTime, n.ElapsedTime, n.Rows, n.Line ) 
}

func displayNodes( current DebugNodeInterface, stack string,  args []string ) {	
	if len(args) == 0 ||  current.match( args )  {
		fmt.Println( current.printNode(stack) ) 
	}
	for _, node := range current.getNode().Childs {
		displayNodes( node, stack + node.getStackName(), args )
	}
}

func displayMeasures( current DebugNodeInterface, args []string ) {
	if len(args) == 0 ||  current.match( args )  {
		fmt.Println( current.printMeasures() ) 
	}
	for _, node := range current.getNode().Childs {
		displayMeasures( node, args )
	}
}
func parseLog ( log string) (debugTree * RootNode) {
	var timestamp uint64
	var rowData []string
	rowIndex := uint32(0)
	// var err error
	root := &RootNode{DebugNode{ DebugBaseNode { "ROOT", 0, 0, 0, 0, rowIndex, 0, []DebugNodeInterface{} }, nil  }, 0,0,0,0,0,0,0,0 } 
	var current DebugNodeInterface
   	current = root
	rows := strings.Split(string(log), "\n")[1:]
	re := regexp.MustCompile("[0-9]+")

	for _, row := range rows {
		rowData = strings.Split(string(row), "|")

		if ( len(rowData) > 1 ) {
	        timestampf, err := strconv.ParseFloat( re.FindString( strings.Split(rowData[0], " ")[1] ) , 10 )
			if err != nil {
				ErrorAndExit( row + ":\n" + err.Error())
			}
	        timestamp = uint64(timestampf)

	       	eventType := rowData[1]
	       	level := 1

	        switch( eventType  ) {        	
	        	case "EXECUTION_STARTED":
	        		root.StartTime = uint64(timestamp)
	       		
	        	case "CODE_UNIT_STARTED", "METHOD_ENTRY", "SYSTEM_METHOD_ENTRY":        		
				// 17:01:49.056 (56251266)|CODE_UNIT_STARTED|[EXTERNAL]|06612000001qh7a|VF: /apex/FieloPRM_VFP_Page			
					var newCode * CodeNode
	    			var idleTime uint64
	    			n := current.getNode()
	    			var lastChild = len(n.Childs) - 1 
	    			if ( lastChild >= 0  ) {
	    				l := n.Childs[lastChild].getNode()
	    				if ( l.EventType == "CODE" ) {
							idleTime = uint64(timestamp) - l.EndTime
	    				}    				
	    			}				
	        		var SubType = strings.Replace( strings.Replace( eventType, "_STARTED", "", 1), "_ENTRY", "", 1)
	        		if len(rowData) == 5  {
	        			newCode = &CodeNode { DebugNode{ DebugBaseNode {"CODE", uint64(timestamp), 0, 0, 0, rowIndex, 0, []DebugNodeInterface{} }, current}, SubType, "", rowData[4], rowData[3], idleTime, level }
	    			} else {
	        			newCode = &CodeNode { DebugNode{ DebugBaseNode {"CODE", uint64(timestamp), 0, 0, 0, rowIndex, 0, []DebugNodeInterface{} }, current}, SubType, "", rowData[3], "", idleTime, level }
	    			}
		
	        		current.addChild( newCode )
	        		current = newCode
	        		level++;

	        	case "SOQL_EXECUTE_BEGIN": 
	        	// 17:01:50.421 (1421361727)|SOQL_EXECUTE_BEGIN|[98]|Aggregations:0|SELECT Name, FieloEE__ExternalName__c FROM Menu__c WHERE Id = :tmpVar1
	        		aggregations, err := strconv.ParseFloat(re.FindString(rowData[3]), 10 )
					if err != nil {
						ErrorAndExit( row + ":\n" + err.Error())
					}
	        		newSOQL  := &SOQLNode { DebugNode{ DebugBaseNode {"SOQL", uint64(timestamp), 0, 0, 0, rowIndex, 0, nil}, current}, rowData[2], uint16(aggregations), rowData[4], 0 }
	        		current.addChild( newSOQL)
	        		current = newSOQL

	        	case "HEAP_ALLOCATE":
					// 14:32:33.434 (17434285698)|HEAP_ALLOCATE|[72]|Bytes:3
	        		bytes, err := strconv.ParseFloat(re.FindString(rowData[3]), 10)
					if err != nil {
						ErrorAndExit( row + ":\n" + err.Error())
					}        		
	        		newHeap  := &HeapNode { DebugNode{ DebugBaseNode {"HEAP", uint64(timestamp), 0, 0, 0, rowIndex, rowIndex, nil}, current}, rowData[2], uint32(bytes) }
	        		current.addChild( newHeap )
	        		// Actualiza las Stats
			    	root.HEAPSize += newHeap.Bytes

	        	case "DML_BEGIN": 
	        	// 17:01:50.444 (1444795717)|DML_BEGIN|[139]|Op:Upsert|Type:FieloEE__FrontEndSessionData__c|Rows:1
	        		numRows , err := strconv.ParseFloat(re.FindString(rowData[5]), 10)
					if err != nil {
						ErrorAndExit( row + ":\n" + err.Error())
					}        		
	        		newDML  := &DMLNode { DebugNode{ DebugBaseNode {"DML", uint64(timestamp), 0, 0, 0, rowIndex, 0, nil }, current}, rowData[2] ,rowData[3], rowData[4], uint16(numRows) }
					current.addChild( newDML )
	        		current = newDML

	        	case "EXECUTION_FINISHED", "CODE_UNIT_FINISHED", "METHOD_EXIT", "SYSTEM_METHOD_EXIT", "SOQL_EXECUTE_END", "DML_END":        		
	        		current.setEndTime( uint64(timestamp), rowIndex )
	        		// Add rows & Actualiza Stats
	        		switch( eventType ) {
						case "SOQL_EXECUTE_END":
		        			numRows, err := strconv.ParseFloat(re.FindString(rowData[3]), 10)
							if err != nil {
								ErrorAndExit( row + ":\n" + err.Error())
							}
						    sqlNode, _ := current.(*SOQLNode)
    						sqlNode.Rows = uint16(numRows)
					    	root.SQOLNum++
					    	root.SQOLRows += sqlNode.Rows
					    	root.SQOLTime += sqlNode.ElapsedTime
						case "DML_END":
						    dmlNode, _ := current.(*DMLNode)
					    	root.DMLNum++
					    	root.DMLRows += dmlNode.Rows
					    	root.DMLTime += dmlNode.ElapsedTime
						default:
						    currentNode := current.getNode()
						    root.ExecTime += (currentNode.ElapsedTime - currentNode.ChildElapseTime)
	        		}
		    		current = current.getNode().Parent
		    		level++;
		    
	        	case "LIMIT_USAGE_FOR_NS":
/*
  Number of SOQL queries: 24 out of 100
  Number of query rows: 65 out of 50000
  Number of SOSL queries: 0 out of 20
  Number of DML statements: 2 out of 150
  Number of DML rows: 2 out of 10000
  Maximum CPU time: 561 out of 10000
  Maximum heap size: 0 out of 6000000
  Number of callouts: 0 out of 100
  Number of Email Invocations: 0 out of 10
  Number of future calls: 0 out of 50
  Number of queueable jobs added to the queue: 0 out of 50
  Number of Mobile Apex push calls: 0 out of 10
*/
	        }
    	}
    	rowIndex++
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
		where := getWhereCondition(args[2:])		
		records, err := force.QueryLogs(where)
		if err != nil {
			ErrorAndExit(err.Error())
		}
		DisplayForceRecordsf(records.Records, format)		
	} else if args[0] == "delete" {
		where := getWhereCondition(args[1:])

		result, err := force.Query("SELECT Id FROM ApexLog " + where, true )
		if err != nil {
			ErrorAndExit(err.Error())
		}

		for _, record := range result.Records {
			err := force.DeleteToolingRecord("ApexLog", record["Id"].(string) )
			if err != nil {
				ErrorAndExit(err.Error())
			}
		}	
		fmt.Printf("Debug Logs deleted\n")

	} else if args[0] == "download" {
		where := getWhereCondition(args[1:])
		result, err := force.Query("SELECT Id, Operation, Status, DurationMilliseconds FROM ApexLog " + where + " ORDER BY StartTime DESC", true )
		if err != nil {
			ErrorAndExit(err.Error())
		}

		i := 1
		for _, record := range result.Records {
			id := record["Id"].(string)
			//operation := record["Operation"].(string)
			log, err := force.RetrieveLog(id)
			if err != nil {
				ErrorAndExit(err.Error())
			}
			// operation
			filename := fmt.Sprintf( "%v.log", i )
			err = ioutil.WriteFile( filename, []byte(log), 0644)
			if err != nil {
				ErrorAndExit(err.Error())
			}
			i++
		}

	} else { // 
		var log string
		var file []byte
		var err error
		if args[0][0:3] == "07L"  {
			log, err = force.RetrieveLog(args[0])
		} else {			
			file, err = ioutil.ReadFile(args[2])
			log = string(file)
		}
		if err != nil {
			ErrorAndExit(err.Error())
		}

		root := parseLog ( log )

		switch format {
			case "csv", "csv-noheader":
				eventType := ""				
				for _, value := range args {
					options := strings.Split(value, ":")
					if len(options) == 2  &&  strings.ToLower(options[0]) == "eventtype"  {
						eventType = strings.ToLower(options[1]) 
						break;
					}
				}
				if format == "csv" {
					switch ( eventType ) {
						case "root":
							fmt.Println(  "\"EventType\",\"SQOLRows\",\"SQOLTime\",\"SQOLNum\",\"DMLRows\",\"DMLTime\",\"DMLNum\",\"ExecTime\",\"HEAPSize\",\"TotalTime\"" )							
						case "code":
							fmt.Println(  "\"Stack\",\"CodeUnit\",\"StartTime\",\"EndTime\",\"ElapsedTime\",\"IdleTime\",\"Line\",\"Level\"" )
						case "heap":
							fmt.Println(  "\"Stack\",\"StartTime\",\"Bytes\",\"Line\"" )
						case "soql":
							fmt.Println(  "\"Stack\",\"SOQL\",\"StartTime\",\"EndTime\",\"ElapsedTime\",\"Rows\",\"Aggregations\",\"Line\"" )
						case "dml":
							fmt.Println(  "\"Stack\",\"Operation\",\"Type\",\"StartTime\",\"EndTime\",\"ElapsedTime\",\"Rows\",\"Line\"" )
						default:
							fmt.Println( "\"Event\",\"Reference\",\"Variable\",\"Measure\"" )
					}
				}
				if ( eventType == "" ) {
					displayMeasures( root, args[3:] )
				} else {
					displayNodes( root, "", args[3:] )
				}

			case "json":
				json.Marshal(root.DebugBaseNode )

			case "json-pretty":
				json.MarshalIndent(root.DebugBaseNode, "", "  ")

			case "text":
				fmt.Println(log)
			default:
				fmt.Printf("Format %s not supported\n\n", format)
		}

	}
}
