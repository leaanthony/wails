package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/internal/s"
)

const versionFile = "../../internal/version/version.txt"
const changelogFile = "docs/src/content/docs/changelog.mdx"

// Command-line flags
var (
	checkOnly bool
	dryRun    bool
)

func checkError(err error) {
	if err != nil {
		println(err.Error())
		os.Exit(1)
	}
}

// printUsage prints command-line usage information
func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: release.go [--check-only] [--dry-run] [version]\n")
	fmt.Fprintf(os.Stderr, "  --check-only  Check if unreleased content exists (exit 0 if yes, 1 if no)\n")
	fmt.Fprintf(os.Stderr, "  --dry-run     Show what would be done without making changes\n")
	fmt.Fprintf(os.Stderr, "  version       Specific version to release (optional, auto-increments if not specified)\n")
	fmt.Fprintf(os.Stderr, "\nNote: --check-only exits immediately and ignores --dry-run if both are specified.\n")
}

// readVersion reads and validates the current version from the version file
func readVersion() string {
	currentVersionData, err := os.ReadFile(versionFile)
	checkError(err)
	currentVersion := strings.TrimSpace(string(currentVersionData))
	if currentVersion == "" {
		checkError(fmt.Errorf("version file %s is empty", versionFile))
	}
	if !strings.Contains(currentVersion, ".") {
		checkError(fmt.Errorf("version %q in %s does not contain a dot", currentVersion, versionFile))
	}
	return currentVersion
}

// incrementVersion takes a version string and returns the incremented version
func incrementVersion(currentVersion string) string {
	vsplit := strings.Split(currentVersion, ".")
	if len(vsplit) == 0 {
		checkError(fmt.Errorf("failed to parse version %q", currentVersion))
	}
	minorVersion, err := strconv.Atoi(vsplit[len(vsplit)-1])
	checkError(err)
	minorVersion++
	vsplit[len(vsplit)-1] = strconv.Itoa(minorVersion)
	return strings.Join(vsplit, ".")
}

// hasUnreleasedContent checks if the changelog has content in the Unreleased section.
// This function changes the working directory to the repo root.
func hasUnreleasedContent() (bool, error) {
	// We need to be in the repo root to read the changelog
	s.CD("../../..")

	changelogData, err := os.ReadFile(changelogFile)
	if err != nil {
		return false, fmt.Errorf("failed to read changelog file %s: %w", changelogFile, err)
	}
	changelog := string(changelogData)

	// Split on the Unreleased header
	parts := strings.Split(changelog, "## [Unreleased]")
	if len(parts) < 2 {
		return false, nil // No Unreleased section found
	}

	// Extract the unreleased section content
	releaseNotes := extractReleaseNotes(parts[1])

	// Check if there's any meaningful content
	return len(strings.TrimSpace(releaseNotes)) > 0, nil
}

// parseArgs parses command-line arguments and returns the version if specified
func parseArgs() string {
	var version string

	for _, arg := range os.Args[1:] {
		switch arg {
		case "--check-only":
			checkOnly = true
		case "--dry-run":
			dryRun = true
		default:
			// Check if it looks like a flag we don't recognize
			if strings.HasPrefix(arg, "--") || strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "Unknown flag: %s\n", arg)
				printUsage()
				os.Exit(1)
			}
			// Ensure only a single version argument is provided
			if version != "" {
				fmt.Fprintf(os.Stderr, "Multiple version arguments provided: %s and %s\n", version, arg)
				printUsage()
				os.Exit(1)
			}
			version = arg
		}
	}

	return version
}

// TODO:This can be replaced with "https://github.com/coreos/go-semver/blob/main/semver/semver.go"
func updateVersion() string {
	currentVersion := readVersion()
	newVersion := incrementVersion(currentVersion)
	err := os.WriteFile(versionFile, []byte(newVersion), 0o755)
	checkError(err)
	return newVersion
}

//func runCommand(name string, arg ...string) {
//	cmd := exec.Command(name, arg...)
//	cmd.Stdout = os.Stdout
//	cmd.Stderr = os.Stderr
//	err := cmd.Run()
//	checkError(err)
//}

// extractReleaseNotes extracts just the content from the Unreleased section
// until the next version header, removing empty lines from the beginning and end
func extractReleaseNotes(changelogSection string) string {
	// Find the next version header (starts with "## v" or "## [")
	lines := strings.Split(changelogSection, "\n")
	var releaseLines []string

	for _, line := range lines {
		// Stop at the next version header
		if strings.HasPrefix(strings.TrimSpace(line), "## v") || strings.HasPrefix(strings.TrimSpace(line), "## [") {
			break
		}
		releaseLines = append(releaseLines, line)
	}

	// Join lines and trim empty lines from beginning and end
	releaseNotes := strings.Join(releaseLines, "\n")
	releaseNotes = strings.TrimSpace(releaseNotes)

	return releaseNotes
}

//func IsPointRelease(currentVersion string, newVersion string) bool {
//	// The first n-1 parts of the version should be the same
//	if currentVersion[:len(currentVersion)-2] != newVersion[:len(newVersion)-2] {
//		return false
//	}
//	// split on the last dot in the string
//	currentVersionSplit := strings.Split(currentVersion, ".")
//	newVersionSplit := strings.Split(newVersion, ".")
//	// if the last part of the version is the same, it's a point release
//	currentMinor := lo.Must(strconv.Atoi(currentVersionSplit[len(currentVersionSplit)-1]))
//	newMinor := lo.Must(strconv.Atoi(newVersionSplit[len(newVersionSplit)-1]))
//	return newMinor == currentMinor+1
//}

func main() {
	// Parse command-line arguments first
	specifiedVersion := parseArgs()

	// Handle --check-only mode: just check for unreleased content and exit
	// Note: --check-only takes precedence over --dry-run
	if checkOnly {
		hasContent, err := hasUnreleasedContent()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking changelog: %v\n", err)
			os.Exit(1)
		}
		if hasContent {
			println("Unreleased changelog content found")
			os.Exit(0)
		} else {
			println("No unreleased changelog content")
			os.Exit(1)
		}
	}

	// Determine the new version
	var newVersion string
	if specifiedVersion != "" {
		newVersion = specifiedVersion
		if dryRun {
			println("[DRY RUN] Would write version to " + versionFile + ": " + newVersion)
		} else {
			err := os.WriteFile(versionFile, []byte(newVersion), 0o755)
			checkError(err)
		}
	} else {
		currentVersion := readVersion()
		newVersion = incrementVersion(currentVersion)
		if dryRun {
			println("[DRY RUN] Would increment version from " + currentVersion + " to " + newVersion)
		} else {
			err := os.WriteFile(versionFile, []byte(newVersion), 0o755)
			checkError(err)
		}
	}

	// Update ChangeLog
	s.CD("../../..")

	// Read in changelog
	changelogData, err := os.ReadFile(changelogFile)
	checkError(err)
	changelog := string(changelogData)
	// Split on the line that has `## [Unreleased]`
	changelogSplit := strings.Split(changelog, "## [Unreleased]")
	// Get today's date in YYYY-MM-DD format
	today := time.Now().Format("2006-01-02")

	// Extract release notes from the Unreleased section
	releaseNotes := extractReleaseNotes(changelogSplit[1])

	// Print JUST the release notes for version tag
	println("=== RELEASE NOTES FOR " + newVersion + " ===")
	print(releaseNotes)
	println("=== END RELEASE NOTES ===")

	// Add the new version to the top of the changelog
	newChangelog := changelogSplit[0] + "## [Unreleased]\n\n## " + newVersion + " - " + today + changelogSplit[1]
	// Write the changelog back
	if dryRun {
		println("[DRY RUN] Would update changelog at " + changelogFile)
	} else {
		err = os.WriteFile(changelogFile, []byte(newChangelog), 0o755)
		checkError(err)
	}

	// TODO: Documentation Versioning and Translations

	//if !isPointRelease {
	//	runCommand("npx", "-y", "pnpm", "install")
	//
	//	s.ECHO("Generating new Docs for version: " + newVersion)
	//
	//	runCommand("npx", "pnpm", "run", "docusaurus", "docs:version", newVersion)
	//
	//	runCommand("npx", "pnpm", "run", "write-translations")
	//
	//	// Load the version list/*
	//	versionsData, err := os.ReadFile("versions.json")
	//	checkError(err)
	//	var versions []string
	//	err = json.Unmarshal(versionsData, &versions)
	//	checkError(err)
	//	oldestVersion := versions[len(versions)-1]
	//	s.ECHO(oldestVersion)
	//	versions = versions[0 : len(versions)-1]
	//	newVersions, err := json.Marshal(&versions)
	//	checkError(err)
	//	err = os.WriteFile("versions.json", newVersions, 0o755)
	//	checkError(err)
	//
	//	s.ECHO("Removing old version: " + oldestVersion)
	//	s.CD("versioned_docs")
	//	s.RMDIR("version-" + oldestVersion)
	//	s.CD("../versioned_sidebars")
	//	s.RM("version-" + oldestVersion + "-sidebars.json")
	//	s.CD("..")
	//
	//	runCommand("npx", "pnpm", "run", "build")
	//}
}
