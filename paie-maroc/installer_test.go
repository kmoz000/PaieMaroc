package main

import (
	"os"
	"strings"
	"testing"
)

func TestWindowsUserInstallerUsesPerUserStorage(t *testing.T) {
	t.Parallel()

	project, err := os.ReadFile("build/windows/installer/project.nsi")
	if err != nil {
		t.Fatal(err)
	}
	installer := string(project)
	for _, expected := range []string{
		`!if "${WAILS_INSTALL_SCOPE}" == "user"`,
		`InstallDir "$LOCALAPPDATA\Programs\${INFO_PRODUCTNAME}"`,
	} {
		if !strings.Contains(installer, expected) {
			t.Fatalf("project.nsi does not contain %q", expected)
		}
	}

	tools, err := os.ReadFile("build/windows/installer/wails_tools.nsh")
	if err != nil {
		t.Fatal(err)
	}
	registry := string(tools)
	for _, expected := range []string{
		`WriteRegStr HKCU "${UNINST_KEY}" "DisplayName"`,
		`WriteRegDWORD HKCU "${UNINST_KEY}" "EstimatedSize"`,
		`DeleteRegKey HKCU "${UNINST_KEY}"`,
	} {
		if !strings.Contains(registry, expected) {
			t.Fatalf("wails_tools.nsh does not contain %q", expected)
		}
	}
}
