package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"l4d2-mod-join/internal/modscan"
	"l4d2-mod-join/internal/vpkmerge"
)

func TestPrepareOfficialWeaponSoundOverlaysAddsMissingGameSounds(t *testing.T) {
	root := t.TempDir()
	gameDir := filepath.Join(root, "Left 4 Dead 2")
	left4dead2 := filepath.Join(gameDir, "left4dead2")
	addons := filepath.Join(left4dead2, "addons")
	source := filepath.Join(root, "workshop")
	output := filepath.Join(root, "merged")
	for _, dir := range []string{left4dead2, addons, source, output} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	officialWAV := filepath.Join(root, "official.wav")
	if err := os.WriteFile(officialWAV, testPCM16WAV([]int16{1000}), 0644); err != nil {
		t.Fatal(err)
	}
	weaponScript, gameSoundScript := writeOfficialSoundScripts(t, root, "pistol", "Weapon_Pistol.Single", "sound/weapons/pistol/gunfire/pistol_fire_1.wav")
	if err := vpkmerge.Run(vpkmerge.Plan{
		Output: left4dead2,
		Groups: []vpkmerge.Group{{
			Output: "pak01_dir.vpk",
			Title:  "Official",
			Overlay: map[string]string{
				"sound/weapons/pistol/gunfire/pistol_fire_1.wav": officialWAV,
				"sound/weapons/melee/swing.wav":                  officialWAV,
				"scripts/weapon_pistol.txt":                      weaponScript,
				"scripts/game_sounds_weapons.txt":                gameSoundScript,
			},
		}},
	}, nil); err != nil {
		t.Fatal(err)
	}
	result := modscan.Result{
		Directory: source,
		Categories: []modscan.Category{{
			Key: "weapons", Output: "04_Weapons.vpk", Title: "Merged Weapons", Packages: []string{"model-only.vpk"},
		}},
		MissingWeaponSounds: []modscan.MissingWeaponSound{{
			Package: "model-only.vpk",
			Weapons: []string{"pistol"},
		}},
	}
	plan := vpkmerge.Plan{
		Input:  source,
		Output: output,
		Groups: []vpkmerge.Group{{
			Output: "04_Weapons.vpk",
			Title:  "Merged Weapons",
		}},
	}
	volume := 50
	applyWeaponSoundVolume(&plan, volume)
	cleanup, added, err := prepareOfficialWeaponSoundOverlays(&plan, &result, addons, volume)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatal(err)
	}
	if added != 1 {
		t.Fatalf("added = %d, want 1", added)
	}
	if err := vpkmerge.Run(plan, nil); err != nil {
		t.Fatal(err)
	}
	content, err := vpkmerge.ReadFile(filepath.Join(output, "04_Weapons.vpk"), "sound/weapons/pistol/gunfire/pistol_fire_1.wav")
	if err != nil {
		t.Fatal(err)
	}
	if got := int16(binary.LittleEndian.Uint16(content[len(content)-2:])); got != 500 {
		t.Fatalf("official pistol sample = %d, want 500", got)
	}
	if _, err := vpkmerge.ReadFile(filepath.Join(output, "04_Weapons.vpk"), "sound/weapons/melee/swing.wav"); err == nil {
		t.Fatal("non-shooting official weapon sound should not be injected")
	}
}

func TestPrepareOfficialWeaponSoundOverlaysDoesNotOverrideModSound(t *testing.T) {
	root := t.TempDir()
	gameDir := filepath.Join(root, "Left 4 Dead 2")
	left4dead2 := filepath.Join(gameDir, "left4dead2")
	addons := filepath.Join(left4dead2, "addons")
	source := filepath.Join(root, "workshop")
	output := filepath.Join(root, "merged")
	for _, dir := range []string{left4dead2, addons, source, output} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	officialWAV := filepath.Join(root, "official.wav")
	modWAV := filepath.Join(root, "mod.wav")
	if err := os.WriteFile(officialWAV, testPCM16WAV([]int16{1000}), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(modWAV, testPCM16WAV([]int16{2000}), 0644); err != nil {
		t.Fatal(err)
	}
	weaponScript, gameSoundScript := writeOfficialSoundScripts(t, root, "pistol", "Weapon_Pistol.Single", "sound/weapons/pistol/gunfire/pistol_fire_1.wav")
	if err := vpkmerge.Run(vpkmerge.Plan{
		Output: left4dead2,
		Groups: []vpkmerge.Group{{
			Output: "pak01_dir.vpk",
			Title:  "Official",
			Overlay: map[string]string{
				"sound/weapons/pistol/gunfire/pistol_fire_1.wav": officialWAV,
				"scripts/weapon_pistol.txt":                      weaponScript,
				"scripts/game_sounds_weapons.txt":                gameSoundScript,
			},
		}},
	}, nil); err != nil {
		t.Fatal(err)
	}
	if err := vpkmerge.Run(vpkmerge.Plan{
		Output: source,
		Groups: []vpkmerge.Group{{
			Output: "model-and-sound.vpk",
			Title:  "Mod",
			Overlay: map[string]string{
				"sound/weapons/pistol/gunfire/pistol_fire_1.wav": modWAV,
			},
		}},
	}, nil); err != nil {
		t.Fatal(err)
	}
	result := modscan.Result{
		Directory: source,
		Categories: []modscan.Category{{
			Key: "weapons", Output: "04_Weapons.vpk", Title: "Merged Weapons", Packages: []string{"model-and-sound.vpk"},
		}},
		MissingWeaponSounds: []modscan.MissingWeaponSound{{
			Package: "model-and-sound.vpk",
			Weapons: []string{"pistol"},
		}},
	}
	plan, err := result.Plan(output, nil)
	if err != nil {
		t.Fatal(err)
	}
	volume := 50
	applyWeaponSoundVolume(&plan, volume)
	cleanup, added, err := prepareOfficialWeaponSoundOverlays(&plan, &result, addons, volume)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatal(err)
	}
	if added != 0 {
		t.Fatalf("official overlay should not replace mod sound, added=%d", added)
	}
	if err := vpkmerge.Run(plan, nil); err != nil {
		t.Fatal(err)
	}
	content, err := vpkmerge.ReadFile(filepath.Join(output, "04_Weapons.vpk"), "sound/weapons/pistol/gunfire/pistol_fire_1.wav")
	if err != nil {
		t.Fatal(err)
	}
	if got := int16(binary.LittleEndian.Uint16(content[len(content)-2:])); got != 1000 {
		t.Fatalf("mod pistol sample = %d, want 1000", got)
	}
}

func TestPrepareOfficialWeaponSoundOverlaysRequiresOfficialVPK(t *testing.T) {
	root := t.TempDir()
	addons := filepath.Join(root, "Left 4 Dead 2", "left4dead2", "addons")
	if err := os.MkdirAll(addons, 0755); err != nil {
		t.Fatal(err)
	}
	result := modscan.Result{
		Categories: []modscan.Category{{
			Key: "weapons", Output: "04_Weapons.vpk", Title: "Merged Weapons", Packages: []string{"model-only.vpk"},
		}},
		MissingWeaponSounds: []modscan.MissingWeaponSound{{
			Package: "model-only.vpk",
			Weapons: []string{"pumpshotgun"},
		}},
	}
	plan := vpkmerge.Plan{Groups: []vpkmerge.Group{{
		Output: "04_Weapons.vpk",
		Title:  "Merged Weapons",
	}}}
	_, _, err := prepareOfficialWeaponSoundOverlays(&plan, &result, addons, 50)
	if err == nil || !strings.Contains(err.Error(), "游戏 Addons 目录") {
		t.Fatalf("expected actionable official VPK error, got %v", err)
	}
}

func TestPrepareOfficialWeaponSoundOverlaysErrorsWhenOfficialSoundMissing(t *testing.T) {
	root := t.TempDir()
	gameDir := filepath.Join(root, "Left 4 Dead 2")
	left4dead2 := filepath.Join(gameDir, "left4dead2")
	addons := filepath.Join(left4dead2, "addons")
	if err := os.MkdirAll(addons, 0755); err != nil {
		t.Fatal(err)
	}
	officialWAV := filepath.Join(root, "official.wav")
	if err := os.WriteFile(officialWAV, testPCM16WAV([]int16{1000}), 0644); err != nil {
		t.Fatal(err)
	}
	weaponScript, gameSoundScript := writeOfficialSoundScripts(t, root, "pistol", "Weapon_Pistol.Single", "sound/weapons/pistol/gunfire/pistol_fire_1.wav")
	if err := vpkmerge.Run(vpkmerge.Plan{
		Output: left4dead2,
		Groups: []vpkmerge.Group{{
			Output: "pak01_dir.vpk",
			Title:  "Official",
			Overlay: map[string]string{
				"sound/weapons/pistol/gunfire/pistol_fire_1.wav": officialWAV,
				"scripts/weapon_pistol.txt":                      weaponScript,
				"scripts/game_sounds_weapons.txt":                gameSoundScript,
			},
		}},
	}, nil); err != nil {
		t.Fatal(err)
	}
	result := modscan.Result{
		MissingWeaponSounds: []modscan.MissingWeaponSound{{
			Package: "model-only.vpk",
			Weapons: []string{"pumpshotgun"},
		}},
	}
	plan := vpkmerge.Plan{Groups: []vpkmerge.Group{{
		Output: "04_Weapons.vpk",
		Title:  "Merged Weapons",
	}}}
	_, _, err := prepareOfficialWeaponSoundOverlays(&plan, &result, addons, 50)
	if err == nil || !strings.Contains(err.Error(), "pumpshotgun") {
		t.Fatalf("expected missing official sound error, got %v", err)
	}
}

func writeOfficialSoundScripts(t *testing.T, root, weapon, event, wave string) (string, string) {
	t.Helper()
	weaponScript := filepath.Join(root, "weapon_"+weapon+".txt")
	gameSoundScript := filepath.Join(root, "game_sounds_weapons.txt")
	weaponContent := "\"WeaponData\"\r\n{\r\n\t\"SoundData\"\r\n\t{\r\n\t\t\"single_shot\"\t\"" + event + "\"\r\n\t}\r\n}\r\n"
	gameSoundContent := "\"" + event + "\"\r\n{\r\n\t\"channel\"\t\"CHAN_WEAPON\"\r\n\t\"wave\"\t\"" + wave + "\"\r\n}\r\n"
	if err := os.WriteFile(weaponScript, []byte(weaponContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gameSoundScript, []byte(gameSoundContent), 0644); err != nil {
		t.Fatal(err)
	}
	return weaponScript, gameSoundScript
}

func testPCM16WAV(samples []int16) []byte {
	dataSize := len(samples) * 2
	out := make([]byte, 44+dataSize)
	copy(out[0:4], "RIFF")
	binary.LittleEndian.PutUint32(out[4:8], uint32(36+dataSize))
	copy(out[8:12], "WAVE")
	copy(out[12:16], "fmt ")
	binary.LittleEndian.PutUint32(out[16:20], 16)
	binary.LittleEndian.PutUint16(out[20:22], 1)
	binary.LittleEndian.PutUint16(out[22:24], 1)
	binary.LittleEndian.PutUint32(out[24:28], 44100)
	binary.LittleEndian.PutUint32(out[28:32], 44100*2)
	binary.LittleEndian.PutUint16(out[32:34], 2)
	binary.LittleEndian.PutUint16(out[34:36], 16)
	copy(out[36:40], "data")
	binary.LittleEndian.PutUint32(out[40:44], uint32(dataSize))
	for i, sample := range samples {
		binary.LittleEndian.PutUint16(out[44+i*2:46+i*2], uint16(sample))
	}
	return out
}
