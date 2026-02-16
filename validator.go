package vanillampq

import (
	"fmt"
	"strings"
)

// VanillaVersion represents a vanilla WoW client version
type VanillaVersion string

const (
	Version100 VanillaVersion = "1.0.0"  // Launch
	Version110 VanillaVersion = "1.1.0"
	Version120 VanillaVersion = "1.2.0"
	Version130 VanillaVersion = "1.3.0"
	Version140 VanillaVersion = "1.4.0"
	Version150 VanillaVersion = "1.5.0"
	Version160 VanillaVersion = "1.6.0"
	Version170 VanillaVersion = "1.7.0"
	Version180 VanillaVersion = "1.8.0"
	Version190 VanillaVersion = "1.9.0"
	Version1100 VanillaVersion = "1.10.0"
	Version1110 VanillaVersion = "1.11.0"
	Version1120 VanillaVersion = "1.12.0"
	Version1121 VanillaVersion = "1.12.1" // Final vanilla patch
)

// VanillaArchiveType represents the type of vanilla MPQ archive
type VanillaArchiveType string

const (
	TypeBase     VanillaArchiveType = "base"     // Base game archives (e.g., dbc.MPQ, interface.MPQ)
	TypePatch    VanillaArchiveType = "patch"    // Patch archives (e.g., patch.MPQ, patch-2.MPQ)
	TypeLocale   VanillaArchiveType = "locale"   // Locale-specific archives
	TypeCustom   VanillaArchiveType = "custom"   // Custom/addon archives
)

// KnownVanillaArchives lists the standard MPQ archives found in vanilla WoW clients
var KnownVanillaArchives = map[string]VanillaArchiveType{
	"base.MPQ":      TypeBase,
	"dbc.MPQ":       TypeBase,
	"fonts.MPQ":     TypeBase,
	"interface.MPQ": TypeBase,
	"misc.MPQ":      TypeBase,
	"model.MPQ":     TypeBase,
	"sound.MPQ":     TypeBase,
	"speech.MPQ":    TypeBase,
	"terrain.MPQ":   TypeBase,
	"texture.MPQ":   TypeBase,
	"wmo.MPQ":       TypeBase,
	
	"patch.MPQ":     TypePatch,
	"patch-2.MPQ":   TypePatch,
	"patch-3.MPQ":   TypePatch,
	
	"locale-enUS.MPQ": TypeLocale,
	"locale-enGB.MPQ": TypeLocale,
	"locale-deDE.MPQ": TypeLocale,
	"locale-frFR.MPQ": TypeLocale,
	"locale-koKR.MPQ": TypeLocale,
	"locale-zhCN.MPQ": TypeLocale,
	"locale-zhTW.MPQ": TypeLocale,
	"locale-esES.MPQ": TypeLocale,
	"locale-esMX.MPQ": TypeLocale,
	"locale-ruRU.MPQ": TypeLocale,
}

// IsVanillaArchive checks if an archive name is a known vanilla archive
func IsVanillaArchive(name string) bool {
	_, ok := KnownVanillaArchives[name]
	return ok
}

// GetArchiveType returns the type of a vanilla archive
func GetArchiveType(name string) VanillaArchiveType {
	if archiveType, ok := KnownVanillaArchives[name]; ok {
		return archiveType
	}
	return TypeCustom
}

// IsVanillaVersion checks if a version string is within the vanilla range
func IsVanillaVersion(version string) bool {
	// Simple check: version should start with "1."
	return strings.HasPrefix(version, "1.") && !strings.HasPrefix(version, "1.13")
}

// ValidateArchiveName validates that an archive name follows vanilla conventions
func ValidateArchiveName(name string) error {
	if name == "" {
		return fmt.Errorf("archive name cannot be empty")
	}
	
	// Check file extension
	if !strings.HasSuffix(strings.ToLower(name), ".mpq") {
		return fmt.Errorf("archive must have .MPQ extension")
	}
	
	return nil
}
