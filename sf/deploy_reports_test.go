package sf

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// deployTestsRan is the result of a deploy that ran two tests, one of which
// failed, with half of one class covered.
func deployTestsRan(status string, testErrors int) string {
	failures, successes := "", `<successes><id>01p000000000002</id><methodName>doubles</methodName><name>CalcTest</name><namespace xsi:nil="true"/><time>26.0</time></successes>`
	if testErrors > 0 {
		failures = `<failures><id>01p000000000002</id><message>System.AssertException: Assertion Failed: not five</message><methodName>fails</methodName><name>CalcTest</name><namespace xsi:nil="true"/><stackTrace>Class.CalcTest.fails: line 9, column 1</stackTrace><time>80.0</time><type>Class</type></failures>`
	} else {
		successes += `<successes><id>01p000000000002</id><methodName>fails</methodName><name>CalcTest</name><namespace xsi:nil="true"/><time>75.0</time></successes>`
	}
	return `<checkOnly>false</checkOnly>` +
		`<details>` +
		`<componentSuccesses><changed>true</changed><componentType>ApexClass</componentType><created>false</created><deleted>false</deleted><fileName>classes/Greeter.cls</fileName><fullName>Greeter</fullName><id>01p000000000001AAA</id><success>true</success></componentSuccesses>` +
		`<runTestResult>` +
		`<codeCoverage><id>01p000000000001</id><locationsNotCovered><column>0</column><line>5</line><numExecutions>0</numExecutions><time>-1.0</time></locationsNotCovered><locationsNotCovered><column>0</column><line>6</line><numExecutions>0</numExecutions><time>-1.0</time></locationsNotCovered><name>Calc</name><namespace xsi:nil="true"/><numLocations>4</numLocations><numLocationsNotCovered>2</numLocationsNotCovered><type>Class</type></codeCoverage>` +
		`<codeCoverageWarnings><id>01p000000000001</id><message>Test coverage of selected Apex Class is 50%, at least 75% test coverage is required</message><name>Calc</name><namespace xsi:nil="true"/></codeCoverageWarnings>` +
		failures +
		`<numFailures>` + string(rune('0'+testErrors)) + `</numFailures><numTestsRun>2</numTestsRun>` + successes + `<totalTime>161.0</totalTime></runTestResult>` +
		`</details>` +
		`<done>true</done><id>0Af000000000001AAA</id>` +
		`<numberComponentErrors>0</numberComponentErrors><numberComponentsDeployed>1</numberComponentsDeployed><numberComponentsTotal>1</numberComponentsTotal>` +
		`<numberTestErrors>` + string(rune('0'+testErrors)) + `</numberTestErrors><numberTestsCompleted>` + string(rune('0'+2-testErrors)) + `</numberTestsCompleted><numberTestsTotal>2</numberTestsTotal>` +
		`<rollbackOnError>true</rollbackOnError><runTestsEnabled>true</runTestsEnabled><status>` + status + `</status><success>` + map[bool]string{true: "true", false: "false"}[status == "Succeeded"] + `</success>`
}

const coverageWarningOutput = " ›   Warning: Calc - Test coverage of selected Apex Class is 50%, at least 75% \n ›   test coverage is required\n"

func TestProjectDeployStartPrintsTestFailures(t *testing.T) {
	_, fake, _ := deploySetup(t)
	fake.statusResult = deployTestsRan("Failed", 1)
	out, errOut, status := run("project", "deploy", "start", "-d", "force-app/main/default/classes", "-l", "RunSpecifiedTests", "-t", "CalcTest")
	if status != 1 || errOut != coverageWarningOutput {
		t.Errorf("status %d, stderr %q", status, errOut)
	}
	want := ` ✔ Deploying Metadata T
   ▸ Components: 1/1 (100%)
 ✘ Running Tests T
   ▸ Successful: 1/2 (50%)
   ▸ Failed: 1/2 (50%)
 ◼ Updating Source Tracking
 ◼ Done

 Status: Failed
 Deploy ID: 0Af000000000001AAA
 Target Org: me@example.com
 Elapsed Time: T


Test Failures [1]
• CalcTest.fails
  message: System.AssertException: Assertion Failed: not five
  stacktrace: 
    Class.CalcTest.fails: line 9, column 1


Test Results Summary
Passing: 1
Failing: 1
Total: 2
Time: 161
`
	if got := durationPattern.ReplaceAllString(out, "T"); !strings.HasSuffix(got, want) {
		t.Errorf("output =\n%s\nwant suffix\n%s", got, want)
	}
}

func TestProjectDeployStartVerbose(t *testing.T) {
	_, fake, _ := deploySetup(t)
	fake.statusResult = deployTestsRan("Succeeded", 0)
	out, errOut, status := run("project", "deploy", "start", "-d", "force-app/main/default/classes", "-l", "RunSpecifiedTests", "-t", "CalcTest", "--verbose")
	if status != 0 || errOut != coverageWarningOutput {
		t.Fatalf("status %d, stderr %q\n%s", status, errOut, out)
	}
	if !strings.Contains(out, "✓ CalcTest.doubles (26ms)\n") {
		t.Errorf("output lacks the time of a test:\n%s", out)
	}
	got := regexp.MustCompile(`Size: [0-9.]+ (B|KB)`).ReplaceAllString(durationPattern.ReplaceAllString(out, "T"), "Size: N")
	for _, want := range []string{
		" Target Org: me@example.com\n Deploy URL:\n http://127.0.0.1:",
		"\n Size: N of ~39 MB limit\n Files: 5 of 10,000 limit\n Elapsed Time: T\n",
		`

Test Success [2]
✓ CalcTest.doubles (T)
✓ CalcTest.fails (T)

Apex Code Coverage
┌──────┬───────────┬─────────────────┐
│ Name │ % Covered │ Uncovered Lines │
├──────┼───────────┼─────────────────┤
│ Calc │ 50%       │ 5,6             │
└──────┴───────────┴─────────────────┘


Test Results Summary
Passing: 2
Failing: 0
Total: 2
Time: 161
`} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}
	// No line of the progress display is longer than 80 characters.
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, " http") && len(line) > 80 {
			t.Errorf("line of %d characters: %s", len(line), line)
		}
	}
}

func TestProjectDeployStartWritesCoverageAndJUnitFiles(t *testing.T) {
	_, fake, project := deploySetup(t)
	fake.statusResult = deployTestsRan("Failed", 1)
	out, _, status := run("project", "deploy", "start", "-d", "force-app/main/default/classes", "-l", "RunSpecifiedTests", "-t", "CalcTest", "--junit",
		"--coverage-formatters", "json-summary", "--coverage-formatters", "text", "--coverage-formatters", "cobertura", "--coverage-formatters", "lcovonly", "--coverage-formatters", "clover",
		"--coverage-formatters", "json", "--coverage-formatters", "teamcity", "--coverage-formatters", "text-summary", "--coverage-formatters", "none", "--results-dir", "res")
	if status != 1 || !strings.HasSuffix(out, "Code Coverage formats, [json-summary, text, cobertura, lcovonly, clover, json, teamcity, text-summary, none], written to res/coverage/\nJunit results written to res/junit/junit.xml\n") {
		t.Fatalf("status %d\n%s", status, out)
	}
	numbers := regexp.MustCompile(`(timestamp|generated)="[^"]+"`)
	read := func(name string) string {
		return numbers.ReplaceAllString(readFile(t, filepath.Join(project, "res", filepath.FromSlash(name))), `$1="N"`)
	}
	for name, want := range map[string]string{
		"coverage/text.txt": `----------|---------|----------|---------|---------|-------------------
File      | % Stmts | % Branch | % Funcs | % Lines | Uncovered Line #s 
----------|---------|----------|---------|---------|-------------------
All files |      50 |      100 |     100 |      50 |                   
 Calc     |      50 |      100 |     100 |      50 | 5-6               
----------|---------|----------|---------|---------|-------------------
`,
		"coverage/coverage-summary.json": `{"total": {"lines":{"total":4,"covered":2,"skipped":0,"pct":50},"statements":{"total":4,"covered":2,"skipped":0,"pct":50},"functions":{"total":0,"covered":0,"skipped":0,"pct":100},"branches":{"total":0,"covered":0,"skipped":0,"pct":100},"branchesTrue":{"total":0,"covered":0,"skipped":0,"pct":"Unknown"}}
,"no-map/Calc": {"lines":{"total":4,"covered":2,"skipped":0,"pct":50},"functions":{"total":0,"covered":0,"skipped":0,"pct":100},"statements":{"total":4,"covered":2,"skipped":0,"pct":50},"branches":{"total":0,"covered":0,"skipped":0,"pct":100}}
}
`,
		"coverage/lcovonly.info": "TN:\nSF:no-map/Calc\nFNF:0\nFNH:0\nDA:5,0\nDA:6,0\nDA:7,1\nDA:8,1\nLF:4\nLH:2\nBRF:0\nBRH:0\nend_of_record\n",
		"coverage/cobertura.xml": `<?xml version="1.0" ?>
<!DOCTYPE coverage SYSTEM "http://cobertura.sourceforge.net/xml/coverage-04.dtd">
<coverage lines-valid="4" lines-covered="2" line-rate="0.5" branches-valid="0" branches-covered="0" branch-rate="1" timestamp="N" complexity="0" version="0.1">
  <sources>
    <source>.</source>
  </sources>
  <packages>
    <package name="main" line-rate="0.5" branch-rate="1">
      <classes>
        <class name="Calc" filename="no-map/Calc" line-rate="0.5" branch-rate="1">
          <methods>
          </methods>
          <lines>
            <line number="5" hits="0" branch="false"/>
            <line number="6" hits="0" branch="false"/>
            <line number="7" hits="1" branch="false"/>
            <line number="8" hits="1" branch="false"/>
          </lines>
        </class>
      </classes>
    </package>
  </packages>
</coverage>
`,
		"coverage/clover.xml": `<?xml version="1.0" encoding="UTF-8"?>
<coverage generated="N" clover="3.2.0">
  <project timestamp="N" name="All files">
    <metrics statements="4" coveredstatements="2" conditionals="0" coveredconditionals="0" methods="0" coveredmethods="0" elements="4" coveredelements="2" complexity="0" loc="4" ncloc="4" packages="1" files="1" classes="1"/>
    <file name="Calc" path="no-map/Calc">
      <metrics statements="4" coveredstatements="2" conditionals="0" coveredconditionals="0" methods="0" coveredmethods="0"/>
      <line num="5" count="0" type="stmt"/>
      <line num="6" count="0" type="stmt"/>
      <line num="7" count="1" type="stmt"/>
      <line num="8" count="1" type="stmt"/>
    </file>
  </project>
</coverage>
`,
		"coverage/coverage.json": `{"no-map/Calc": {"fnMap":{},"branchMap":{},"path":"no-map/Calc","f":{},"b":{},"s":{"5":0,"6":0,"7":1,"8":1},"statementMap":{"5":{"start":{"line":5,"column":0},"end":{"line":5,"column":0}},"6":{"start":{"line":6,"column":0},"end":{"line":6,"column":0}},"7":{"start":{"line":7,"column":0},"end":{"line":7,"column":0}},"8":{"start":{"line":8,"column":0},"end":{"line":8,"column":0}}}}
}
`,
		"coverage/teamcity.txt":     "\n##teamcity[blockOpened name='coverage']\n##teamcity[buildStatisticValue key='CodeCoverageAbsBCovered' value='2']\n##teamcity[buildStatisticValue key='CodeCoverageAbsBTotal' value='4']\n##teamcity[buildStatisticValue key='CodeCoverageAbsRCovered' value='0']\n##teamcity[buildStatisticValue key='CodeCoverageAbsRTotal' value='0']\n##teamcity[buildStatisticValue key='CodeCoverageAbsMCovered' value='0']\n##teamcity[buildStatisticValue key='CodeCoverageAbsMTotal' value='0']\n##teamcity[buildStatisticValue key='CodeCoverageAbsLCovered' value='2']\n##teamcity[buildStatisticValue key='CodeCoverageAbsLTotal' value='4']\n##teamcity[blockClosed name='coverage']\n",
		"coverage/text-summary.txt": "\n=============================== Coverage summary ===============================\nStatements   : 50% ( 2/4 )\nBranches     : 100% ( 0/0 )\nFunctions    : 100% ( 0/0 )\nLines        : 50% ( 2/4 )\n================================================================================\n",
	} {
		if got := read(name); got != want {
			t.Errorf("%s =\n%s\nwant\n%s", name, got, want)
		}
	}
	junit := regexp.MustCompile(`testStartTime" value="[^"]+"`).ReplaceAllString(read("junit/junit.xml"), `testStartTime" value="X"`)
	wantJUnit := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
    <testsuite name="force.apex" timestamp="N" hostname="HOST" tests="2" failures="1"  errors="0"  time="0.16">
        <properties>
            <property name="commandTime" value="0.00 s"/>
            <property name="failRate" value="50.00%"/>
            <property name="failing" value="1"/>
            <property name="hostname" value="HOST"/>
            <property name="orgId" value="00D000000000000"/>
            <property name="passRate" value="50.00%"/>
            <property name="passing" value="1"/>
            <property name="skipped" value="0"/>
            <property name="testExecutionTime" value="0.16 s"/>
            <property name="testStartTime" value="X"/>
            <property name="testTotalTime" value="0.16 s"/>
            <property name="testsRan" value="2"/>
            <property name="username" value="me@example.com"/>
        </properties>
        <testcase name="doubles" classname="CalcTest" time="0.03">
        </testcase>
        <testcase name="fails" classname="CalcTest" time="0.08">
            <failure message="System.AssertException: Assertion Failed: not five"><![CDATA[Class.CalcTest.fails: line 9, column 1]]></failure>
        </testcase>
    </testsuite>
</testsuites>
`
	junit = regexp.MustCompile(`http://127\.0\.0\.1:\d+/?`).ReplaceAllString(junit, "HOST")
	if junit != wantJUnit {
		t.Errorf("junit.xml =\n%s\nwant\n%s", junit, wantJUnit)
	}

	// With --json the files are written and the result is typed as sf
	// types it.
	os.RemoveAll(filepath.Join(project, "res"))
	out, _, status = run("project", "deploy", "start", "-d", "force-app/main/default/classes", "-l", "RunSpecifiedTests", "-t", "CalcTest", "--junit", "--json")
	if status != 1 || !exists(filepath.Join(project, "coverage", "junit", "junit.xml")) {
		t.Errorf("json: status %d\n%s", status, out)
	}
	for _, want := range []string{`"locationsNotCovered": [
              {
                "column": 0,
                "line": 5,
                "numExecutions": 0,
                "time": -1
              },`, `            "namespace": null,
            "numLocations": 4,
            "numLocationsNotCovered": 2,
            "type": "Class",
            "dmlInfo": [],
            "methodInfo": [],
            "soqlInfo": [],
            "soslInfo": []`} {
		if !strings.Contains(out, want) {
			t.Errorf("json lacks %s:\n%s", want, out)
		}
	}
}

func TestDeployReportFlagsAreChecked(t *testing.T) {
	deploySetup(t)
	for _, c := range []struct {
		args   []string
		status int
		want   string
	}{
		{[]string{"--results-dir", "res"}, 2, "Error (2): The following error occurred:\n  One of the following must be provided when using --results-dir: --coverage-formatters, --junit\nSee more help with --help\n\n\n"},
		{[]string{"--coverage-formatters", "nope"}, 1, "Error (1): Expected --coverage-formatters=nope to be one of: clover, cobertura, html-spa, html, json, json-summary, lcovonly, none, teamcity, text, text-summary\nSee more help with --help\n\n\n"},
		{[]string{"--coverage-formatters", "html"}, 1, "Error (UnsupportedFlagError): The html coverage format is not supported by the sf-compatible mode of force.\n\n\n"},
	} {
		_, errOut, status := run(append([]string{"project", "deploy", "start", "-d", "force-app"}, c.args...)...)
		if status != c.status || errOut != c.want {
			t.Errorf("%v: status %d, stderr %q", c.args, status, errOut)
		}
	}
	// The flags are given without tests: sf warns and writes no JUnit file.
	_, errOut, status := run("project", "deploy", "start", "-d", "force-app/main/default/classes", "--junit")
	if status != 0 || errOut != "Warning: `--junit` was specified but no tests ran.\nYou can ensure tests run by specifying `--test-level` and setting it to `RunSpecifiedTests`, `RunLocalTests` or `RunAllTestsInOrg`.\n" {
		t.Errorf("no tests: status %d, stderr %q", status, errOut)
	}
}

func TestProjectDeployValidateReportsAFailedValidation(t *testing.T) {
	_, fake, _ := deploySetup(t)
	fake.statusResult = deployTestsRan("Failed", 1)
	out, errOut, status := run("project", "deploy", "validate", "-d", "force-app/main/default/classes", "-l", "RunSpecifiedTests", "-t", "CalcTest")
	wantErr := "Error (FailedValidationError): Failed to validate the deployment (0Af000000000001AAA). Due To:\nCalc - Test coverage of selected Apex Class is 50%, at least 75% test coverage is required\n\n\nCalcTest.fails - System.AssertException: Assertion Failed: not five\n\n\n"
	if status != 1 || !strings.HasSuffix(errOut, wantErr) || !strings.Contains(out, "Test Results Summary\n") {
		t.Errorf("status %d, stderr %q\n%s", status, errOut, out)
	}
	out, _, status = run("project", "deploy", "validate", "-d", "force-app/main/default/classes", "-l", "RunSpecifiedTests", "-t", "CalcTest", "--json")
	for _, want := range []string{`"name": "FailedValidationError"`, `"deployId": "0Af000000000001AAA"`, `"context": "DeployMetadataValidate"`} {
		if status != 1 || !strings.Contains(out, want) {
			t.Errorf("json lacks %s: status %d\n%s", want, status, out)
		}
	}
}
