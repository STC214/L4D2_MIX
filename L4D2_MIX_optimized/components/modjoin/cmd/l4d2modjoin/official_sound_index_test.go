package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseOfficialSoundScriptsResolveWeaponWAVs(t *testing.T) {
	gameSounds := parseGameSoundScript(`
"Weapon_Pistol.Single"
{
	"channel" "CHAN_WEAPON"
	"rndwave"
	{
		"wave" ")weapons/pistol/gunfire/pistol_fire_1.wav"
		"wave" "weapons/pistol/gunfire/pistol_fire_2.wav"
	}
}
"Weapon_Melee.Swing"
{
	"wave" "weapons/melee/swing.wav"
}
`)
	weaponEvents := parseWeaponSoundEvents(`
"WeaponData"
{
	"SoundData"
	{
		"single_shot" "Weapon_Pistol.Single"
	}
}
`)
	index := officialWeaponSoundIndex{
		ByWeapon: map[string][]string{},
		Location: map[string]string{
			"sound/weapons/pistol/gunfire/pistol_fire_1.wav": "pak01_dir.vpk",
			"sound/weapons/pistol/gunfire/pistol_fire_2.wav": "pak01_dir.vpk",
		},
	}
	for event := range weaponEvents {
		for path := range gameSounds[event] {
			index.ByWeapon["pistol"] = append(index.ByWeapon["pistol"], path)
		}
	}
	resolved, err := requireOfficialWeaponSounds(index, []string{"pistol"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved) != 2 {
		t.Fatalf("resolved = %#v, want two pistol WAVs", resolved)
	}
	if resolved["sound/weapons/pistol/gunfire/pistol_fire_1.wav"] == "" {
		t.Fatalf("prefixed wave was not normalized: %#v", resolved)
	}
	if _, ok := resolved["sound/weapons/melee/swing.wav"]; ok {
		t.Fatalf("melee sound should not be resolved as shooting weapon sound: %#v", resolved)
	}
}

func TestRequireOfficialWeaponSoundsReportsUnresolvedWeapons(t *testing.T) {
	_, err := requireOfficialWeaponSounds(officialWeaponSoundIndex{
		ByWeapon: map[string][]string{},
		Location: map[string]string{},
	}, []string{"pumpshotgun"})
	if err == nil {
		t.Fatal("expected unresolved official sound error")
	}
}

func TestOfficialSoundEventsUseLaterSearchPathOverride(t *testing.T) {
	events := map[string]map[string]bool{}
	replaceSoundEvents(events, parseGameSoundScript(`
"Weapon_Pistol.Single"
{
	"wave" "weapons/pistol/gunfire/base.wav"
}
`))
	replaceSoundEvents(events, parseGameSoundScript(`
"Weapon_Pistol.Single"
{
	"wave" "weapons/pistol/gunfire/update.wav"
}
`))
	if events["Weapon_Pistol.Single"]["sound/weapons/pistol/gunfire/base.wav"] {
		t.Fatalf("base event path should be replaced by later search path: %#v", events)
	}
	if !events["Weapon_Pistol.Single"]["sound/weapons/pistol/gunfire/update.wav"] {
		t.Fatalf("update event path missing after replacement: %#v", events)
	}
}

func TestResolveLooseOfficialSoundLocationsPrefersHigherPriorityRoot(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "left4dead2")
	update := filepath.Join(root, "update")
	soundPath := filepath.FromSlash("sound/weapons/pistol/gunfire/pistol_fire.wav")
	for _, dir := range []string{filepath.Join(base, filepath.Dir(soundPath)), filepath.Join(update, filepath.Dir(soundPath))} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	baseSound := filepath.Join(base, soundPath)
	updateSound := filepath.Join(update, soundPath)
	if err := os.WriteFile(baseSound, []byte("base"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(updateSound, []byte("update"), 0644); err != nil {
		t.Fatal(err)
	}
	index := officialWeaponSoundIndex{
		ByWeapon: map[string][]string{"pistol": []string{"sound/weapons/pistol/gunfire/pistol_fire.wav"}},
		Location: map[string]string{},
	}
	resolveLooseOfficialSoundLocations(&index, []string{
		filepath.Join(base, "pak01_dir.vpk"),
		filepath.Join(update, "pak01_dir.vpk"),
	})
	if got := index.Location["sound/weapons/pistol/gunfire/pistol_fire.wav"]; got != updateSound {
		t.Fatalf("location = %q, want higher-priority update sound %q", got, updateSound)
	}
}
