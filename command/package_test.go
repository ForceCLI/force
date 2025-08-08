package command

import (
	"testing"
)

func Test_package_version_create_command_requires_flags(t *testing.T) {
	cmd := packageVersionCreateCmd

	// Test that required flags are marked as required
	requiredFlags := []string{"package-id", "version-number", "version-name", "version-description"}

	for _, flagName := range requiredFlags {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("Flag %s not found", flagName)
			continue
		}

		// Check if flag is marked as required
		annotations := flag.Annotations
		if annotations == nil {
			t.Errorf("Flag %s is not marked as required", flagName)
			continue
		}
		if values, ok := annotations["cobra_annotation_bash_completion_one_required_flag"]; !ok || len(values) == 0 || values[0] != "true" {
			t.Errorf("Flag %s is not marked as required", flagName)
		}
	}
}

func Test_package_version_create_command_has_optional_flags(t *testing.T) {
	cmd := packageVersionCreateCmd

	// Test that optional flags exist
	optionalFlags := []string{"ancestor-id", "skip-validation", "async-validation", "code-coverage"}

	for _, flagName := range optionalFlags {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("Flag %s not found", flagName)
		}
	}
}

func Test_package_version_create_command_accepts_one_argument(t *testing.T) {
	cmd := packageVersionCreateCmd

	// Test that command expects exactly one argument (the path)
	if cmd.Args == nil {
		t.Error("Command does not have Args validation")
		return
	}

	// Test with no arguments
	err := cmd.Args(cmd, []string{})
	if err == nil {
		t.Error("Command should require an argument")
	}

	// Test with one argument
	err = cmd.Args(cmd, []string{"/path/to/source"})
	if err != nil {
		t.Error("Command should accept one argument")
	}

	// Test with two arguments
	err = cmd.Args(cmd, []string{"/path/to/source", "extra"})
	if err == nil {
		t.Error("Command should not accept more than one argument")
	}
}

func Test_package_version_release_command_requires_version_id_flag(t *testing.T) {
	cmd := packageVersionReleaseCmd

	// Test that version-id flag is marked as required
	flag := cmd.Flags().Lookup("version-id")
	if flag == nil {
		t.Error("Flag version-id not found")
		return
	}

	// Check if flag is marked as required
	annotations := flag.Annotations
	if annotations == nil {
		t.Error("Flag version-id is not marked as required")
		return
	}
	if values, ok := annotations["cobra_annotation_bash_completion_one_required_flag"]; !ok || len(values) == 0 || values[0] != "true" {
		t.Error("Flag version-id is not marked as required")
	}
}

func Test_package_version_release_command_accepts_no_arguments(t *testing.T) {
	cmd := packageVersionReleaseCmd

	// Test that command expects no arguments
	if cmd.Args == nil {
		t.Error("Command does not have Args validation")
		return
	}

	// Test with no arguments
	err := cmd.Args(cmd, []string{})
	if err != nil {
		t.Error("Command should accept no arguments")
	}

	// Test with one argument
	err = cmd.Args(cmd, []string{"extra"})
	if err == nil {
		t.Error("Command should not accept arguments")
	}
}

func Test_package_version_list_command_has_optional_flags(t *testing.T) {
	cmd := packageVersionListCmd

	// Test that optional flags exist
	optionalFlags := []string{"package-id", "released", "verbose"}

	for _, flagName := range optionalFlags {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("Flag %s not found", flagName)
		}
	}
}

func Test_package_version_list_command_accepts_no_arguments(t *testing.T) {
	cmd := packageVersionListCmd

	// Test that command expects no arguments
	if cmd.Args == nil {
		t.Error("Command does not have Args validation")
		return
	}

	// Test with no arguments
	err := cmd.Args(cmd, []string{})
	if err != nil {
		t.Error("Command should accept no arguments")
	}

	// Test with one argument
	err = cmd.Args(cmd, []string{"extra"})
	if err == nil {
		t.Error("Command should not accept arguments")
	}
}
