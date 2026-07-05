package main

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestOfficialWeaponSoundIndexFromInstalledVPK(t *testing.T) {
	vpkPath := os.Getenv("L4D2_OFFICIAL_VPK")
	if vpkPath == "" {
		t.Skip("set L4D2_OFFICIAL_VPK to validate against an installed game VPK")
	}
	vpks := officialGameVPKs(filepath.Join(filepath.Dir(vpkPath), "addons"))
	if len(vpks) == 0 {
		vpks = []string{vpkPath}
	}
	index, err := loadOfficialWeaponSoundIndex(vpks)
	if err != nil {
		t.Fatal(err)
	}
	var weapons []string
	for weapon := range shootingWeaponScriptNames {
		weapons = append(weapons, weapon)
	}
	sort.Strings(weapons)
	for _, weapon := range weapons {
		paths, err := requireOfficialWeaponSounds(index, []string{weapon})
		if err != nil {
			t.Fatalf("%s: %v", weapon, err)
		}
		if len(paths) == 0 {
			t.Fatalf("%s resolved no official weapon WAV paths", weapon)
		}
	}
}
