package sf

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

// coverageFormats are the values sf accepts for --coverage-formatters.
var coverageFormats = []string{"clover", "cobertura", "html-spa", "html", "json", "json-summary", "lcovonly", "none", "teamcity", "text", "text-summary"}

// classCoverage is the line coverage of one class or trigger in a deploy
// result.
type classCoverage struct {
	name string
	// lines maps a line number to 1 when it is covered and 0 when not.
	lines map[int]int
}

func (c *classCoverage) path() string {
	return "no-map/" + c.name
}

func (c *classCoverage) sortedLines() []int {
	lines := make([]int, 0, len(c.lines))
	for line := range c.lines {
		lines = append(lines, line)
	}
	sort.Ints(lines)
	return lines
}

func (c *classCoverage) counts() (total, covered int) {
	for _, hits := range c.lines {
		total++
		covered += hits
	}
	return total, covered
}

// deployCoverage converts the code coverage of a deploy result.  The result
// names only the lines that are not covered; like sf, this takes the
// covered lines to be the other lines of a run of numLocations lines that
// starts at the first uncovered line, or at line 1.
func deployCoverage(result lib.RunTestResult) []*classCoverage {
	classes := make([]*classCoverage, 0, len(result.CodeCoverage))
	for _, coverage := range result.CodeCoverage {
		class := &classCoverage{name: coverage.Name, lines: map[int]int{}}
		first := 1
		for i, location := range coverage.LocationsNotCovered {
			if i == 0 || location.Line < first {
				first = location.Line
			}
		}
		for line := first; line < first+coverage.NumLocations; line++ {
			class.lines[line] = 1
		}
		for _, location := range coverage.LocationsNotCovered {
			class.lines[location.Line] = 0
		}
		classes = append(classes, class)
	}
	sort.SliceStable(classes, func(i, j int) bool { return classes[i].path() < classes[j].path() })
	return classes
}

// coveragePercent is the percentage istanbul reports: rounded down to two
// decimals, and 100 when there is nothing to cover.
func coveragePercent(covered, total int) float64 {
	if total == 0 {
		return 100
	}
	return math.Floor(float64(covered)*10000/float64(total)) / 100
}

func formatNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func coverageTotals(classes []*classCoverage) (total, covered int) {
	for _, class := range classes {
		t, c := class.counts()
		total += t
		covered += c
	}
	return total, covered
}

// uncoveredRanges lists the uncovered lines the way the text report of
// istanbul does: lines that follow each other among the lines of the class
// are joined into a range.
func (c *classCoverage) uncoveredRanges() string {
	var ranges []string
	lines := c.sortedLines()
	for i := 0; i < len(lines); i++ {
		if c.lines[lines[i]] != 0 {
			continue
		}
		start := i
		for i+1 < len(lines) && c.lines[lines[i+1]] == 0 {
			i++
		}
		if i > start {
			ranges = append(ranges, fmt.Sprintf("%d-%d", lines[start], lines[i]))
		} else {
			ranges = append(ranges, strconv.Itoa(lines[start]))
		}
	}
	return strings.Join(ranges, ",")
}

func coverageSummaryJSON(classes []*classCoverage) string {
	entry := func(total, covered int) string {
		return fmt.Sprintf(`{"total":%d,"covered":%d,"skipped":0,"pct":%s}`, total, covered, formatNumber(coveragePercent(covered, total)))
	}
	const none = `{"total":0,"covered":0,"skipped":0,"pct":100}`
	total, covered := coverageTotals(classes)
	var b strings.Builder
	fmt.Fprintf(&b, `{"total": {"lines":%s,"statements":%s,"functions":%s,"branches":%s,"branchesTrue":{"total":0,"covered":0,"skipped":0,"pct":"Unknown"}}`+"\n", entry(total, covered), entry(total, covered), none, none)
	for _, class := range classes {
		t, c := class.counts()
		fmt.Fprintf(&b, `,%s: {"lines":%s,"functions":%s,"statements":%s,"branches":%s}`+"\n", strconv.Quote(class.path()), entry(t, c), none, entry(t, c), none)
	}
	b.WriteString("}\n")
	return b.String()
}

func coverageJSON(classes []*classCoverage) string {
	var b strings.Builder
	b.WriteString("{")
	for i, class := range classes {
		if i > 0 {
			b.WriteString(",")
		}
		var hits, positions []string
		for _, line := range class.sortedLines() {
			hits = append(hits, fmt.Sprintf(`"%d":%d`, line, class.lines[line]))
			positions = append(positions, fmt.Sprintf(`"%d":{"start":{"line":%d,"column":0},"end":{"line":%d,"column":0}}`, line, line, line))
		}
		fmt.Fprintf(&b, `%s: {"fnMap":{},"branchMap":{},"path":%s,"f":{},"b":{},"s":{%s},"statementMap":{%s}}`+"\n", strconv.Quote(class.path()), strconv.Quote(class.path()), strings.Join(hits, ","), strings.Join(positions, ","))
	}
	b.WriteString("}\n")
	return b.String()
}

func coverageLcov(classes []*classCoverage) string {
	var b strings.Builder
	for _, class := range classes {
		total, covered := class.counts()
		fmt.Fprintf(&b, "TN:\nSF:%s\nFNF:0\nFNH:0\n", class.path())
		for _, line := range class.sortedLines() {
			fmt.Fprintf(&b, "DA:%d,%d\n", line, class.lines[line])
		}
		fmt.Fprintf(&b, "LF:%d\nLH:%d\nBRF:0\nBRH:0\nend_of_record\n", total, covered)
	}
	return b.String()
}

func coverageCobertura(classes []*classCoverage, now time.Time) string {
	rate := func(covered, total int) string {
		return formatNumber(coveragePercent(covered, total) / 100)
	}
	total, covered := coverageTotals(classes)
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" ?>\n<!DOCTYPE coverage SYSTEM \"http://cobertura.sourceforge.net/xml/coverage-04.dtd\">\n")
	fmt.Fprintf(&b, "<coverage lines-valid=\"%d\" lines-covered=\"%d\" line-rate=\"%s\" branches-valid=\"0\" branches-covered=\"0\" branch-rate=\"1\" timestamp=\"%d\" complexity=\"0\" version=\"0.1\">\n", total, covered, rate(covered, total), now.UnixMilli())
	b.WriteString("  <sources>\n    <source>.</source>\n  </sources>\n  <packages>\n")
	fmt.Fprintf(&b, "    <package name=\"main\" line-rate=\"%s\" branch-rate=\"1\">\n      <classes>\n", rate(covered, total))
	for _, class := range classes {
		t, c := class.counts()
		fmt.Fprintf(&b, "        <class name=\"%s\" filename=\"%s\" line-rate=\"%s\" branch-rate=\"1\">\n          <methods>\n          </methods>\n          <lines>\n", xmlEscape(class.name), xmlEscape(class.path()), rate(c, t))
		for _, line := range class.sortedLines() {
			fmt.Fprintf(&b, "            <line number=\"%d\" hits=\"%d\" branch=\"false\"/>\n", line, class.lines[line])
		}
		b.WriteString("          </lines>\n        </class>\n")
	}
	b.WriteString("      </classes>\n    </package>\n  </packages>\n</coverage>\n")
	return b.String()
}

func coverageClover(classes []*classCoverage, now time.Time) string {
	total, covered := coverageTotals(classes)
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	fmt.Fprintf(&b, "<coverage generated=\"%d\" clover=\"3.2.0\">\n  <project timestamp=\"%d\" name=\"All files\">\n", now.UnixMilli(), now.UnixMilli())
	fmt.Fprintf(&b, "    <metrics statements=\"%d\" coveredstatements=\"%d\" conditionals=\"0\" coveredconditionals=\"0\" methods=\"0\" coveredmethods=\"0\" elements=\"%d\" coveredelements=\"%d\" complexity=\"0\" loc=\"%d\" ncloc=\"%d\" packages=\"1\" files=\"%d\" classes=\"%d\"/>\n", total, covered, total, covered, total, total, len(classes), len(classes))
	for _, class := range classes {
		t, c := class.counts()
		fmt.Fprintf(&b, "    <file name=\"%s\" path=\"%s\">\n      <metrics statements=\"%d\" coveredstatements=\"%d\" conditionals=\"0\" coveredconditionals=\"0\" methods=\"0\" coveredmethods=\"0\"/>\n", xmlEscape(class.name), xmlEscape(class.path()), t, c)
		for _, line := range class.sortedLines() {
			fmt.Fprintf(&b, "      <line num=\"%d\" count=\"%d\" type=\"stmt\"/>\n", line, class.lines[line])
		}
		b.WriteString("    </file>\n")
	}
	b.WriteString("  </project>\n</coverage>\n")
	return b.String()
}

func coverageTeamcity(classes []*classCoverage) string {
	total, covered := coverageTotals(classes)
	var b strings.Builder
	b.WriteString("\n##teamcity[blockOpened name='coverage']\n")
	for _, value := range []struct {
		key   string
		count int
	}{{"BCovered", covered}, {"BTotal", total}, {"RCovered", 0}, {"RTotal", 0}, {"MCovered", 0}, {"MTotal", 0}, {"LCovered", covered}, {"LTotal", total}} {
		fmt.Fprintf(&b, "##teamcity[buildStatisticValue key='CodeCoverageAbs%s' value='%d']\n", value.key, value.count)
	}
	b.WriteString("##teamcity[blockClosed name='coverage']\n")
	return b.String()
}

func coverageTextSummary(classes []*classCoverage) string {
	total, covered := coverageTotals(classes)
	line := func(label string, covered, total int) string {
		return fmt.Sprintf("%-13s: %s%% ( %d/%d )\n", label, formatNumber(coveragePercent(covered, total)), covered, total)
	}
	return "\n=============================== Coverage summary ===============================\n" +
		line("Statements", covered, total) + line("Branches", 0, 0) + line("Functions", 0, 0) + line("Lines", covered, total) +
		"================================================================================\n"
}

func coverageText(classes []*classCoverage) string {
	nameWidth, missingWidth := len("All files"), len("Uncovered Line #s")
	for _, class := range classes {
		if width := len(class.name) + 1; width > nameWidth {
			nameWidth = width
		}
		if width := len(class.uncoveredRanges()); width > missingWidth {
			missingWidth = width
		}
	}
	rule := strings.Repeat("-", nameWidth+1) + "|---------|----------|---------|---------|" + strings.Repeat("-", missingWidth+2) + "\n"
	row := func(name, statements, branches, functions, lines, missing string) string {
		return fmt.Sprintf("%-*s | %7s | %8s | %7s | %7s | %-*s \n", nameWidth, name, statements, branches, functions, lines, missingWidth, missing)
	}
	total, covered := coverageTotals(classes)
	percent := formatNumber(coveragePercent(covered, total))
	var b strings.Builder
	b.WriteString(rule)
	b.WriteString(row("File", "% Stmts", "% Branch", "% Funcs", "% Lines", "Uncovered Line #s"))
	b.WriteString(rule)
	b.WriteString(row("All files", percent, "100", "100", percent, ""))
	for _, class := range classes {
		t, c := class.counts()
		percent := formatNumber(coveragePercent(c, t))
		b.WriteString(row(" "+class.name, percent, "100", "100", percent, class.uncoveredRanges()))
	}
	b.WriteString(rule)
	return b.String()
}

// deployJUnit returns the JUnit report sf writes for the tests of a deploy.
func deployJUnit(org *targetOrg, result lib.RunTestResult, now time.Time) string {
	ran, failing := result.NumberOfTestsRun, result.NumberOfFailures
	passRate := "100%"
	if failing != 0 {
		passRate = fmt.Sprintf("%.2f%%", (1-float64(failing)/float64(ran))*100)
	}
	totalTime := fmt.Sprintf("%.2f", float64(result.TotalTime)/1000)
	host := org.Fields.String("instanceUrl")
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<testsuites>\n")
	fmt.Fprintf(&b, "    <testsuite name=\"force.apex\" timestamp=\"%s\" hostname=\"%s\" tests=\"%d\" failures=\"%d\"  errors=\"0\"  time=\"%s\">\n",
		now.UTC().Format("2006-01-02T15:04:05.000Z"), xmlEscape(host), ran, failing, totalTime)
	b.WriteString("        <properties>\n")
	for _, property := range [][2]string{
		{"commandTime", "0.00 s"},
		{"failRate", fmt.Sprintf("%.2f%%", float64(failing)/float64(ran)*100)},
		{"failing", strconv.Itoa(failing)},
		{"hostname", host},
		{"orgId", org.Fields.String("orgId")},
		{"passRate", passRate},
		{"passing", strconv.Itoa(ran - failing)},
		{"skipped", "0"},
		{"testExecutionTime", totalTime + " s"},
		{"testStartTime", now.Local().Format("Mon Jan 02 2006 3:04:05 PM")},
		{"testTotalTime", totalTime + " s"},
		{"testsRan", strconv.Itoa(ran)},
		{"username", org.Username},
	} {
		fmt.Fprintf(&b, "            <property name=\"%s\" value=\"%s\"/>\n", property[0], xmlEscape(property[1]))
	}
	b.WriteString("        </properties>\n")
	testcase := func(name, class string, ms float32) {
		fmt.Fprintf(&b, "        <testcase name=\"%s\" classname=\"%s\" time=\"%.2f\">\n", xmlEscape(name), xmlEscape(class), float64(int(ms))/1000)
	}
	for _, success := range result.TestSuccesses {
		testcase(success.MethodName, success.Name, success.Time)
		b.WriteString("        </testcase>\n")
	}
	for _, failure := range result.TestFailures {
		testcase(failure.MethodName, failure.Name, failure.Time)
		fmt.Fprintf(&b, "            <failure message=\"%s\">", xmlEscape(failure.Message))
		if failure.StackTrace != "" {
			fmt.Fprintf(&b, "<![CDATA[%s]]>", failure.StackTrace)
		}
		b.WriteString("</failure>\n        </testcase>\n")
	}
	b.WriteString("    </testsuite>\n</testsuites>\n")
	return b.String()
}

// deployReportFlags are the report flags of a deploy command.
type deployReportFlags struct {
	formats    []string
	junit      bool
	resultsDir string
}

func addDeployReportFlags(cmd *cobra.Command) {
	cmd.Flags().StringArray("coverage-formatters", nil, "Format of the code coverage results.")
	cmd.Flags().Bool("junit", false, "Output JUnit test results.")
	cmd.Flags().String("results-dir", "", "Output directory for code coverage and JUnit results; defaults to the deploy ID.")
}

// readDeployReportFlags reads and checks --coverage-formatters, --junit and
// --results-dir.
func readDeployReportFlags(cmd *cobra.Command) (*deployReportFlags, error) {
	flags := &deployReportFlags{}
	if cmd.Flags().Lookup("coverage-formatters") == nil {
		return flags, nil
	}
	flags.formats = splitList(cmd, "coverage-formatters")
	flags.junit, _ = cmd.Flags().GetBool("junit")
	flags.resultsDir, _ = cmd.Flags().GetString("results-dir")
	for _, format := range flags.formats {
		if !containsString(coverageFormats, format) {
			return nil, &Error{Message: fmt.Sprintf("Expected --coverage-formatters=%s to be one of: %s\nSee more help with --help", format, strings.Join(coverageFormats, ", ")), ExitCode: 1}
		}
		if format == "html" || format == "html-spa" {
			return nil, newError("UnsupportedFlagError", "The %s coverage format is not supported by the sf-compatible mode of force.", format)
		}
	}
	if flags.resultsDir != "" && len(flags.formats) == 0 && !flags.junit {
		return nil, &Error{Message: "The following error occurred:\n  One of the following must be provided when using --results-dir: --coverage-formatters, --junit\nSee more help with --help", ExitCode: 2}
	}
	if flags.resultsDir == "" {
		flags.resultsDir = "coverage"
	}
	return flags, nil
}

func (f *deployReportFlags) requested() bool {
	return len(f.formats) > 0 || f.junit
}

// write writes the requested coverage and JUnit files for a deploy result.
func (f *deployReportFlags) write(org *targetOrg, result lib.RunTestResult) error {
	now := time.Now()
	classes := deployCoverage(result)
	files := map[string]string{}
	for _, format := range f.formats {
		switch format {
		case "clover":
			files["coverage/clover.xml"] = coverageClover(classes, now)
		case "cobertura":
			files["coverage/cobertura.xml"] = coverageCobertura(classes, now)
		case "json":
			files["coverage/coverage.json"] = coverageJSON(classes)
		case "json-summary":
			files["coverage/coverage-summary.json"] = coverageSummaryJSON(classes)
		case "lcovonly":
			files["coverage/lcovonly.info"] = coverageLcov(classes)
		case "teamcity":
			files["coverage/teamcity.txt"] = coverageTeamcity(classes)
		case "text":
			files["coverage/text.txt"] = coverageText(classes)
		case "text-summary":
			files["coverage/text-summary.txt"] = coverageTextSummary(classes)
		}
	}
	if f.junit && result.NumberOfTestsRun > 0 {
		files["junit/junit.xml"] = deployJUnit(org, result, now)
	}
	for name, content := range files {
		path := filepath.Join(f.resultsDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// uxWarn prints a warning the way the ux.warn of sf does: each line starts
// with " ›   ", the text is wrapped to 75 columns, and a line that was
// broken at a space keeps that space.
func uxWarn(w interface{ Write([]byte) (int, error) }, message string) {
	const width = 75
	text := "Warning: " + message
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		line := ""
		for _, word := range strings.Split(paragraph, " ") {
			switch {
			case line == "":
				line = word
			case len([]rune(line))+1+len([]rune(word)) > width:
				lines = append(lines, line+" ")
				line = word
			default:
				line += " " + word
			}
		}
		lines = append(lines, line)
	}
	for _, line := range lines {
		fmt.Fprintf(w, " ›   %s\n", line)
	}
}
