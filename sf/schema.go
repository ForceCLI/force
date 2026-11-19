package sf

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newSchemaCmd() *cobra.Command {
	return newGroup("schema", "Generate metadata files.",
		newGroup("generate", "Generate metadata files.", newSchemaGenerateTabCmd()),
	)
}

// tabMotifs maps the number of a custom tab style to the value of the
// motif element.  The names, including the space after "Hexagon", are the
// ones sf writes.
var tabMotifs = map[int]string{
	1: "Heart", 2: "Fan", 3: "Sun", 4: "Hexagon ", 5: "Leaf", 6: "Triangle", 7: "Square", 8: "Diamond", 9: "Lightning", 10: "Moon",
	11: "Star", 12: "Circle", 13: "Box", 14: "Hands", 15: "People", 16: "Bank", 17: "Sack", 18: "Form", 19: "Wrench", 20: "Airplane",
	21: "Computer", 22: "Telephone", 23: "Envelope", 24: "Building", 25: "Alarm clock", 26: "Flag", 27: "Laptop", 28: "Cell phone", 29: "PDA", 30: "Radar dish",
	31: "Car", 32: "Factory", 33: "Desk", 34: "Insect", 35: "Microphone", 36: "Train", 37: "Bridge", 38: "Camera", 39: "Telescope", 40: "Credit card",
	41: "Cash", 42: "Treasure chest", 43: "Jewel", 44: "Hammer", 45: "Ticket", 46: "Stamp", 47: "Knight", 48: "Trophy", 49: "CD/DVD", 50: "Big top",
	51: "Apple", 52: "Balls", 53: "Bell", 54: "Boat", 55: "Books", 56: "Bottle", 57: "Building Block", 58: "Caduceus", 59: "Can", 60: "Umbrella",
	61: "Castle", 62: "Chalkboard", 63: "Chip", 64: "Compass", 65: "Cup", 66: "Dice", 67: "Gears", 68: "Globe", 69: "Guitar", 70: "Handsaw",
	71: "Headset", 72: "Helicopter", 73: "Highway Sign", 74: "Hot Air Balloon", 75: "IP Phone", 76: "Keys", 77: "Locked", 78: "Map", 79: "Measuring Tape", 80: "Motorcycle",
	81: "Musical Note", 82: "Whistle", 83: "Pencil", 84: "Presenter", 85: "Real Estate Sign", 86: "Red Cross", 87: "Safe", 88: "Sailboat", 89: "Saxophone", 90: "Scales",
	91: "Shield", 92: "Ship", 93: "Shopping Cart", 94: "Stethoscope", 95: "Stopwatch", 96: "Street Sign", 97: "Thermometer", 98: "Truck", 99: "TV CRT", 100: "TV Widescreen",
}

func newSchemaGenerateTabCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "tab",
		Short:       "Generate the metadata source files for a new custom tab on a custom object.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "GenerateTab"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireProject(cmd); err != nil {
				return err
			}
			object, _ := cmd.Flags().GetString("object")
			directory, _ := cmd.Flags().GetString("directory")
			if object == "" {
				return missingFlagError("object")
			}
			if directory == "" {
				return missingFlagError("directory")
			}
			absolute, err := filepath.Abs(strings.TrimSpace(directory))
			if err != nil {
				return err
			}
			if !strings.HasSuffix(absolute, "tabs") {
				return flagParseError("SfError", "directory", fmt.Sprintf("Invalid tabs path: %s. Choose a folder ending with /tabs in your project.", directory))
			}
			value, _ := cmd.Flags().GetString("icon")
			icon, err := strconv.Atoi(value)
			if err != nil {
				return flagParseError("", "icon", "Expected an integer but received: "+value)
			}
			if err := integerInRange("icon", icon, 1, 100); err != nil {
				return err
			}
			motif := fmt.Sprintf("Custom%d: %s", icon, tabMotifs[icon])
			path := filepath.Join(absolute, object+".tab-meta.xml")
			content := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<CustomTab>\n    <customObject>true</customObject>\n    <motif>" + motif + "</motif>\n</CustomTab>"
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return fileSystemError("open", path, err)
			}
			tab := newOrderedMap()
			tab.Set("customObject", true)
			tab.Set("motif", motif)
			result := newOrderedMap()
			result.Set("tab", tab)
			result.Set("path", path)
			return emit(cmd, result, func(w io.Writer) {
				fmt.Fprintf(w, "%s\n", paint(w, styleGreen, fmt.Sprintf("Generated a tab file at %s.", path)))
			})
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("object", "o", "", "API name of the custom object you're generating a tab for.")
	cmd.Flags().StringP("directory", "d", "", "Path to a \"tabs\" directory that will contain the source files for your new tab.")
	cmd.Flags().StringP("icon", "i", "1", "Number from 1 to 100 that specifies the color scheme and icon for the custom tab.")
	return cmd
}
