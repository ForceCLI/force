package command

import (
	"testing"
)

func Test_package_version_create_command_requires_flags(t *testing.T) {
	cmd := packageVersionCreateCmd

	// Test that required flags are marked as required (package-id is no longer required since namespace is an alternative)
	requiredFlags := []string{"version-number", "version-name", "version-description"}

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

func Test_package_version_create_command_has_package_id_or_namespace(t *testing.T) {
	cmd := packageVersionCreateCmd

	// Test that package-id flag exists
	packageIdFlag := cmd.Flags().Lookup("package-id")
	if packageIdFlag == nil {
		t.Error("Flag package-id not found")
	}

	// Test that namespace flag exists
	namespaceFlag := cmd.Flags().Lookup("namespace")
	if namespaceFlag == nil {
		t.Error("Flag namespace not found")
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

func Test_package_list_command_accepts_no_arguments(t *testing.T) {
	cmd := packageListCmd

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

func Test_package_installed_command_accepts_no_arguments(t *testing.T) {
	cmd := packageInstalledCmd

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

func Test_package_install_command_has_package_version_id_flag(t *testing.T) {
	cmd := packageInstallCmd

	// Test that package-version-id flag exists
	flag := cmd.Flags().Lookup("package-version-id")
	if flag == nil {
		t.Error("Flag package-version-id not found")
	}

	// Test that -i shorthand is available
	if flag.Shorthand != "i" {
		t.Errorf("Expected shorthand 'i' for package-version-id, got '%s'", flag.Shorthand)
	}
}

func Test_package_install_command_has_optional_flags(t *testing.T) {
	cmd := packageInstallCmd

	// Test that optional flags exist
	optionalFlags := []string{"activate", "password", "package-version-id"}

	for _, flagName := range optionalFlags {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("Flag %s not found", flagName)
		}
	}
}

func Test_package_install_command_accepts_variable_arguments(t *testing.T) {
	cmd := packageInstallCmd

	// Test that command has Args validation
	if cmd.Args == nil {
		t.Error("Command does not have Args validation")
		return
	}

	// Test with no arguments (valid when using --package-version-id)
	err := cmd.Args(cmd, []string{})
	if err != nil {
		t.Error("Command should accept no arguments (when using --package-version-id)")
	}

	// Test with two arguments (namespace and version)
	err = cmd.Args(cmd, []string{"namespace", "version"})
	if err != nil {
		t.Error("Command should accept two arguments (namespace and version)")
	}

	// Test with three arguments (namespace, version, and deprecated password)
	err = cmd.Args(cmd, []string{"namespace", "version", "password"})
	if err != nil {
		t.Error("Command should accept three arguments for backward compatibility")
	}

	// Test with four arguments (too many)
	err = cmd.Args(cmd, []string{"namespace", "version", "password", "extra"})
	if err == nil {
		t.Error("Command should not accept more than three arguments")
	}
}

func Test_package_create_command_requires_flags(t *testing.T) {
	cmd := packageCreateCmd

	// Test that required flags are marked as required
	requiredFlags := []string{"name", "type", "namespace"}

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

func Test_package_create_command_has_optional_description_flag(t *testing.T) {
	cmd := packageCreateCmd

	// Test that description flag exists
	flag := cmd.Flags().Lookup("description")
	if flag == nil {
		t.Error("Flag description not found")
	}
}

func Test_package_create_command_accepts_no_arguments(t *testing.T) {
	cmd := packageCreateCmd

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

func Test_package_uninstall_command_requires_package_version_id_flag(t *testing.T) {
	cmd := packageUninstallCmd

	// Test that package-version-id flag exists
	flag := cmd.Flags().Lookup("package-version-id")
	if flag == nil {
		t.Error("Flag package-version-id not found")
		return
	}

	// Test that -i shorthand is available
	if flag.Shorthand != "i" {
		t.Errorf("Expected shorthand 'i' for package-version-id, got '%s'", flag.Shorthand)
	}

	// Check if flag is marked as required
	annotations := flag.Annotations
	if annotations == nil {
		t.Error("Flag package-version-id is not marked as required")
		return
	}
	if values, ok := annotations["cobra_annotation_bash_completion_one_required_flag"]; !ok || len(values) == 0 || values[0] != "true" {
		t.Error("Flag package-version-id is not marked as required")
	}
}

func Test_package_uninstall_command_accepts_no_arguments(t *testing.T) {
	cmd := packageUninstallCmd

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
