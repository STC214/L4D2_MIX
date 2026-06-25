package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"l4d2-mod-join/internal/vpkmerge"
)

type officialWeaponSoundIndex struct {
	ByWeapon map[string][]string
	Location map[string]string
}

var shootingWeaponScriptNames = map[string]bool{
	"autoshotgun":      true,
	"grenade_launcher": true,
	"hunting_rifle":    true,
	"pistol":           true,
	"pistol_magnum":    true,
	"pumpshotgun":      true,
	"rifle":            true,
	"rifle_ak47":       true,
	"rifle_desert":     true,
	"rifle_m60":        true,
	"rifle_sg552":      true,
	"shotgun_chrome":   true,
	"shotgun_spas":     true,
	"smg":              true,
	"smg_mp5":          true,
	"smg_silenced":     true,
	"sniper_awp":       true,
	"sniper_military":  true,
	"sniper_scout":     true,
}

func loadOfficialWeaponSoundIndex(vpks []string) (officialWeaponSoundIndex, error) {
	index := officialWeaponSoundIndex{
		ByWeapon: map[string][]string{},
		Location: map[string]string{},
	}
	soundEvents := map[string]map[string]bool{}
	weaponEvents := map[string]map[string]bool{}
	for _, vpkPath := range vpks {
		info, err := vpkmerge.Inspect(vpkPath)
		if err != nil {
			return index, err
		}
		for _, file := range info.Files {
			path := normalizeVPKPath(file.Path)
			if vpkmerge.IsWeaponSoundWAV(path) {
				index.Location[path] = vpkPath
			}
			switch {
			case isGameSoundScript(path):
				content, readErr := vpkmerge.ReadFile(vpkPath, path)
				if readErr != nil {
					return index, readErr
				}
				replaceSoundEvents(soundEvents, parseGameSoundScript(string(content)))
			case isShootingWeaponScript(path):
				content, readErr := vpkmerge.ReadFile(vpkPath, path)
				if readErr != nil {
					return index, readErr
				}
				weapon := weaponNameFromScript(path)
				weaponEvents[weapon] = parseWeaponSoundEvents(string(content))
			}
		}
	}
	for weapon, events := range weaponEvents {
		paths := map[string]bool{}
		for event := range events {
			for path := range soundEvents[event] {
				paths[path] = true
			}
		}
		index.ByWeapon[weapon] = sortedBoolKeys(paths)
	}
	resolveLooseOfficialSoundLocations(&index, vpks)
	return index, nil
}

func isGameSoundScript(path string) bool {
	base := filepath.Base(normalizeVPKPath(path))
	return strings.HasPrefix(base, "game_sounds") && strings.HasSuffix(base, ".txt")
}

func isShootingWeaponScript(path string) bool {
	weapon := weaponNameFromScript(path)
	return shootingWeaponScriptNames[weapon]
}

func weaponNameFromScript(path string) string {
	base := strings.TrimSuffix(filepath.Base(normalizeVPKPath(path)), ".txt")
	return strings.TrimPrefix(base, "weapon_")
}

func replaceSoundEvents(dst, src map[string]map[string]bool) {
	for event, paths := range src {
		dst[event] = map[string]bool{}
		for path := range paths {
			dst[event][path] = true
		}
	}
}

func parseGameSoundScript(content string) map[string]map[string]bool {
	tokens := tokenizeKeyValues(content)
	events := map[string]map[string]bool{}
	for index := 0; index+1 < len(tokens); index++ {
		if tokens[index+1] != "{" || tokens[index] == "{" || tokens[index] == "}" {
			continue
		}
		next, paths := collectWavePaths(tokens, index+2)
		if len(paths) > 0 {
			events[tokens[index]] = paths
		}
		index = next
	}
	return events
}

func collectWavePaths(tokens []string, start int) (int, map[string]bool) {
	paths := map[string]bool{}
	for index := start; index < len(tokens); index++ {
		switch tokens[index] {
		case "{":
			next, nested := collectWavePaths(tokens, index+1)
			for path := range nested {
				paths[path] = true
			}
			index = next
		case "}":
			return index, paths
		case "wave":
			if index+1 < len(tokens) {
				if path := normalizeSoundScriptWave(tokens[index+1]); vpkmerge.IsWeaponSoundWAV(path) {
					paths[path] = true
				}
				index++
			}
		}
	}
	return len(tokens), paths
}

func parseWeaponSoundEvents(content string) map[string]bool {
	tokens := tokenizeKeyValues(content)
	events := map[string]bool{}
	for index := 0; index+1 < len(tokens); index++ {
		if strings.EqualFold(tokens[index], "SoundData") && tokens[index+1] == "{" {
			next := collectSoundDataEvents(tokens, index+2, events)
			index = next
		}
	}
	return events
}

func collectSoundDataEvents(tokens []string, start int, events map[string]bool) int {
	for index := start; index < len(tokens); index++ {
		switch tokens[index] {
		case "{":
			index = collectSoundDataEvents(tokens, index+1, events)
		case "}":
			return index
		default:
			if index+1 < len(tokens) && tokens[index+1] != "{" && tokens[index+1] != "}" {
				value := tokens[index+1]
				if strings.Contains(value, ".") {
					events[value] = true
				}
				index++
			}
		}
	}
	return len(tokens)
}

func normalizeSoundScriptWave(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimLeft(value, "*#@)>^<")
	value = normalizeVPKPath(value)
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, "sound/") {
		value = "sound/" + value
	}
	return value
}

func tokenizeKeyValues(content string) []string {
	var tokens []string
	for index := 0; index < len(content); {
		ch := content[index]
		if ch <= ' ' {
			index++
			continue
		}
		if ch == '/' && index+1 < len(content) && content[index+1] == '/' {
			index += 2
			for index < len(content) && content[index] != '\n' {
				index++
			}
			continue
		}
		if ch == '/' && index+1 < len(content) && content[index+1] == '*' {
			index += 2
			for index+1 < len(content) && !(content[index] == '*' && content[index+1] == '/') {
				index++
			}
			if index+1 < len(content) {
				index += 2
			}
			continue
		}
		if ch == '{' || ch == '}' {
			tokens = append(tokens, string(ch))
			index++
			continue
		}
		if ch == '"' {
			index++
			var builder strings.Builder
			for index < len(content) {
				if content[index] == '\\' && index+1 < len(content) {
					builder.WriteByte(content[index+1])
					index += 2
					continue
				}
				if content[index] == '"' {
					index++
					break
				}
				builder.WriteByte(content[index])
				index++
			}
			tokens = append(tokens, builder.String())
			continue
		}
		start := index
		for index < len(content) && content[index] > ' ' && content[index] != '{' && content[index] != '}' {
			if content[index] == '/' && index+1 < len(content) && (content[index+1] == '/' || content[index+1] == '*') {
				break
			}
			index++
		}
		if start == index {
			index++
			continue
		}
		tokens = append(tokens, content[start:index])
	}
	return tokens
}

func sortedBoolKeys(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func requireOfficialWeaponSounds(index officialWeaponSoundIndex, weapons []string) (map[string]string, error) {
	paths := map[string]string{}
	var missing []string
	for _, weapon := range weapons {
		found := false
		for _, path := range index.ByWeapon[weapon] {
			if location := index.Location[path]; location != "" {
				paths[path] = location
				found = true
			}
		}
		if !found {
			missing = append(missing, weapon)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("官方脚本没有解析到这些射击武器的 WAV 资源：%s", strings.Join(missing, ", "))
	}
	return paths, nil
}

func resolveLooseOfficialSoundLocations(index *officialWeaponSoundIndex, vpks []string) {
	var roots []string
	seen := map[string]bool{}
	for _, vpkPath := range vpks {
		root := filepath.Dir(vpkPath)
		key := strings.ToLower(root)
		if !seen[key] {
			seen[key] = true
			roots = append(roots, root)
		}
	}
	for _, paths := range index.ByWeapon {
		for _, path := range paths {
			if index.Location[path] != "" {
				continue
			}
			for indexRoot := len(roots) - 1; indexRoot >= 0; indexRoot-- {
				root := roots[indexRoot]
				local := filepath.Join(root, filepath.FromSlash(path))
				if fileExists(local) {
					index.Location[path] = local
					break
				}
			}
		}
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
