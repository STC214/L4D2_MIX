package vpkmerge

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	signature  = 0x55aa1234
	inline     = 0x7fff
	terminator = 0xffff
)

type sourceEntry struct {
	vpk      string
	dataBase int64
	path     string
	crc      uint32
	offset   uint32
	length   uint32
	preload  []byte
	local    string
}

type Group struct {
	Output             string              `json:"output"`
	Title              string              `json:"title"`
	Packages           []string            `json:"packages"`
	Exclude            []string            `json:"exclude"`
	ExcludeByPackage   map[string][]string `json:"exclude_by_package,omitempty"`
	Overlay            map[string]string   `json:"overlay"`
	SoundVolumePercent *int                `json:"sound_volume_percent,omitempty"`
}

type Plan struct {
	Input  string  `json:"input"`
	Output string  `json:"output"`
	Groups []Group `json:"groups"`
}

type Progress struct {
	GroupIndex int
	GroupCount int
	Output     string
	FileCount  int
	Bytes      int64
}

type FileInfo struct {
	Path   string
	CRC    uint32
	Length uint32
}

type PackageInfo struct {
	Path             string
	Digest           string
	RuntimeSignature string
	Files            []FileInfo
}

func Inspect(path string) (PackageInfo, error) {
	entries, err := readVPK(path)
	if err != nil {
		return PackageInfo{}, err
	}
	info := PackageInfo{Path: path, Files: make([]FileInfo, 0, len(entries))}
	for _, entry := range entries {
		info.Files = append(info.Files, FileInfo{Path: entry.path, CRC: entry.crc, Length: entry.length})
	}
	info.Digest, err = digestPackage(path, entries)
	if err != nil {
		return PackageInfo{}, err
	}
	info.RuntimeSignature = runtimeSignature(info.Files)
	return info, nil
}

func ReadFile(vpkPath, wantedPath string) ([]byte, error) {
	entries, err := readVPK(vpkPath)
	if err != nil {
		return nil, err
	}
	wantedPath = normalize(wantedPath)
	for _, entry := range entries {
		if entry.path == wantedPath {
			return readContent(entry)
		}
	}
	return nil, fmt.Errorf("%s not found in %s", wantedPath, filepath.Base(vpkPath))
}

func Verify(path string) (PackageInfo, error) {
	entries, err := readVPK(path)
	if err != nil {
		return PackageInfo{}, err
	}
	info := PackageInfo{Path: path, Files: make([]FileInfo, 0, len(entries))}
	for _, entry := range entries {
		content, err := readContent(entry)
		if err != nil {
			return PackageInfo{}, fmt.Errorf("%s: %w", entry.path, err)
		}
		if crc32.ChecksumIEEE(content) != entry.crc {
			return PackageInfo{}, fmt.Errorf("%s: CRC mismatch", entry.path)
		}
		info.Files = append(info.Files, FileInfo{Path: entry.path, CRC: entry.crc, Length: uint32(len(content))})
	}
	return info, nil
}

func digestFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func digestPackage(directoryVPK string, entries []sourceEntry) (string, error) {
	paths := map[string]bool{directoryVPK: true}
	for _, entry := range entries {
		paths[entry.vpk] = true
	}
	names := make([]string, 0, len(paths))
	for path := range paths {
		names = append(names, path)
	}
	sort.Strings(names)
	hash := sha256.New()
	for _, path := range names {
		digest, err := digestFile(path)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(hash, "%s:%s\n", strings.ToLower(filepath.Base(path)), digest)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func runtimeSignature(files []FileInfo) string {
	filtered := make([]FileInfo, 0, len(files))
	for _, file := range files {
		if file.Path == "addoninfo.txt" || file.Path == "addonimage.jpg" ||
			file.Path == "addonimage.png" || strings.HasPrefix(file.Path, "source files/") {
			continue
		}
		filtered = append(filtered, file)
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Path < filtered[j].Path })
	hash := sha256.New()
	for _, file := range filtered {
		fmt.Fprintf(hash, "%s:%08x:%d\n", file.Path, file.CRC, file.Length)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func cstring(r *bufio.Reader) (string, error) {
	s, err := r.ReadString(0)
	return strings.TrimSuffix(s, "\x00"), err
}

func readVPK(path string) ([]sourceEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var sig, version, treeSize uint32
	for _, value := range []any{&sig, &version, &treeSize} {
		if err := binary.Read(f, binary.LittleEndian, value); err != nil {
			return nil, err
		}
	}
	if sig != signature || (version != 1 && version != 2) {
		return nil, fmt.Errorf("unsupported VPK header")
	}
	headerSize := int64(12)
	if version == 2 {
		var sectionSizes [4]uint32
		for index := range sectionSizes {
			if err := binary.Read(f, binary.LittleEndian, &sectionSizes[index]); err != nil {
				return nil, err
			}
		}
		headerSize = 28
	}
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if treeSize > 512*1024*1024 || headerSize+int64(treeSize) > stat.Size() {
		return nil, fmt.Errorf("%s contains an invalid VPK tree size", path)
	}
	tree := make([]byte, treeSize)
	if _, err := io.ReadFull(f, tree); err != nil {
		return nil, err
	}
	r := bufio.NewReader(bytes.NewReader(tree))
	var entries []sourceEntry
	for {
		ext, err := cstring(r)
		if err != nil {
			return nil, err
		}
		if ext == "" {
			break
		}
		for {
			dir, err := cstring(r)
			if err != nil {
				return nil, err
			}
			if dir == "" {
				break
			}
			for {
				name, err := cstring(r)
				if err != nil {
					return nil, err
				}
				if name == "" {
					break
				}
				var crc uint32
				var preloadSize, archive uint16
				var offset, length uint32
				var end uint16
				for _, value := range []any{&crc, &preloadSize, &archive, &offset, &length, &end} {
					if err := binary.Read(r, binary.LittleEndian, value); err != nil {
						return nil, err
					}
				}
				if end != terminator {
					return nil, fmt.Errorf("%s contains invalid entry terminator", path)
				}
				preload := make([]byte, preloadSize)
				if _, err := io.ReadFull(r, preload); err != nil {
					return nil, err
				}
				file := name
				if ext != " " {
					file += "." + ext
				}
				if strings.TrimSpace(dir) != "" {
					file = strings.Trim(dir, "/\\ ") + "/" + file
				}
				dataPath := path
				dataBase := headerSize + int64(treeSize)
				if archive != inline {
					dataPath = externalArchivePath(path, archive)
					dataBase = 0
				}
				entries = append(entries, sourceEntry{
					vpk: dataPath, dataBase: dataBase,
					path: normalize(file), crc: crc, offset: offset,
					length: length, preload: preload,
				})
			}
		}
	}
	return entries, nil
}

func externalArchivePath(directoryVPK string, archive uint16) string {
	ext := filepath.Ext(directoryVPK)
	base := strings.TrimSuffix(directoryVPK, ext)
	base = strings.TrimSuffix(base, "_dir")
	return fmt.Sprintf("%s_%03d%s", base, archive, ext)
}

func normalize(path string) string {
	return strings.ToLower(strings.ReplaceAll(filepath.ToSlash(path), "\\", "/"))
}

func isMetadata(path string) bool {
	return path == "addoninfo.txt" || path == "addonimage.jpg" || path == "addonimage.png" ||
		strings.HasPrefix(path, "source files/")
}

func readContent(entry sourceEntry) ([]byte, error) {
	if entry.local != "" {
		return os.ReadFile(entry.local)
	}
	f, err := os.Open(entry.vpk)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data := append([]byte(nil), entry.preload...)
	if entry.length == 0 {
		return data, nil
	}
	buf := make([]byte, entry.length)
	if _, err := f.ReadAt(buf, entry.dataBase+int64(entry.offset)); err != nil {
		return nil, err
	}
	return append(data, buf...), nil
}

type treeEntry struct {
	sourceEntry
	ext, dir, name string
	outOffset      uint32
	outLength      uint32
}

func splitPath(path string) (ext, dir, name string) {
	dir = filepath.ToSlash(filepath.Dir(path))
	if dir == "." {
		dir = " "
	}
	base := filepath.Base(path)
	dot := strings.LastIndex(base, ".")
	if dot < 0 {
		return " ", dir, base
	}
	return base[dot+1:], dir, base[:dot]
}

func makeTree(entries []*treeEntry) ([]byte, error) {
	var tree bytes.Buffer
	byExt := map[string]map[string][]*treeEntry{}
	for _, entry := range entries {
		if byExt[entry.ext] == nil {
			byExt[entry.ext] = map[string][]*treeEntry{}
		}
		byExt[entry.ext][entry.dir] = append(byExt[entry.ext][entry.dir], entry)
	}
	exts := sortedKeys(byExt)
	for _, ext := range exts {
		tree.WriteString(ext)
		tree.WriteByte(0)
		dirs := sortedKeys(byExt[ext])
		for _, dir := range dirs {
			tree.WriteString(dir)
			tree.WriteByte(0)
			files := byExt[ext][dir]
			sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
			for _, entry := range files {
				tree.WriteString(entry.name)
				tree.WriteByte(0)
				for _, value := range []any{
					entry.crc, uint16(0), uint16(inline),
					entry.outOffset, entry.outLength, uint16(terminator),
				} {
					if err := binary.Write(&tree, binary.LittleEndian, value); err != nil {
						return nil, err
					}
				}
			}
			tree.WriteByte(0)
		}
		tree.WriteByte(0)
	}
	tree.WriteByte(0)
	return tree.Bytes(), nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func writeGroup(cfg Plan, g Group) (Progress, error) {
	selected := map[string]sourceEntry{}
	excluded := map[string]bool{}
	for _, path := range g.Exclude {
		excluded[normalize(path)] = true
	}
	for _, name := range g.Packages {
		entries, err := readVPK(filepath.Join(cfg.Input, name))
		if err != nil {
			return Progress{}, fmt.Errorf("%s: %w", name, err)
		}
		for _, entry := range entries {
			if isMetadata(entry.path) || excluded[entry.path] || packageExcluded(g, name, entry.path) {
				continue
			}
			selected[entry.path] = entry
		}
	}
	for path, local := range g.Overlay {
		content, err := os.ReadFile(local)
		if err != nil {
			return Progress{}, fmt.Errorf("overlay %s: %w", local, err)
		}
		selected[normalize(path)] = sourceEntry{
			path: normalize(path), local: local, crc: crc32.ChecksumIEEE(content),
		}
	}
	addonInfo := fmt.Sprintf("\"AddonInfo\"\r\n{\r\n  addonSteamAppID \"550\"\r\n  addontitle \"%s\"\r\n  addonversion \"1\"\r\n  addonauthor \"L4D2 Mod Join\"\r\n  addonDescription \"Merged from %d local mods\"\r\n}\r\n", g.Title, len(g.Packages))
	tmpInfo := filepath.Join(cfg.Output, "."+g.Output+".addoninfo.tmp")
	if err := os.MkdirAll(cfg.Output, 0755); err != nil {
		return Progress{}, err
	}
	if err := os.WriteFile(tmpInfo, []byte(addonInfo), 0644); err != nil {
		return Progress{}, err
	}
	defer os.Remove(tmpInfo)
	selected["addoninfo.txt"] = sourceEntry{
		path: "addoninfo.txt", local: tmpInfo, crc: crc32.ChecksumIEEE([]byte(addonInfo)),
	}

	paths := sortedKeys(selected)
	entries := make([]*treeEntry, 0, len(paths))
	var offset uint64
	for _, path := range paths {
		src := selected[path]
		content, err := readOutputContent(src, g)
		if err != nil {
			return Progress{}, fmt.Errorf("%s: %w", path, err)
		}
		if offset+uint64(len(content)) > uint64(^uint32(0)) {
			return Progress{}, fmt.Errorf("%s exceeds VPK v1 4GiB data limit", g.Output)
		}
		src.crc = crc32.ChecksumIEEE(content)
		ext, dir, name := splitPath(path)
		entries = append(entries, &treeEntry{
			sourceEntry: src, ext: ext, dir: dir, name: name,
			outOffset: uint32(offset), outLength: uint32(len(content)),
		})
		offset += uint64(len(content))
	}
	tree, err := makeTree(entries)
	if err != nil {
		return Progress{}, err
	}
	outPath := filepath.Join(cfg.Output, g.Output)
	out, err := os.CreateTemp(cfg.Output, "."+g.Output+".tmp-*")
	if err != nil {
		return Progress{}, err
	}
	tmpPath := out.Name()
	defer os.Remove(tmpPath)
	for _, value := range []uint32{signature, 1, uint32(len(tree))} {
		if err := binary.Write(out, binary.LittleEndian, value); err != nil {
			out.Close()
			return Progress{}, err
		}
	}
	if _, err := out.Write(tree); err != nil {
		out.Close()
		return Progress{}, err
	}
	for _, entry := range entries {
		content, err := readOutputContent(entry.sourceEntry, g)
		if err != nil {
			out.Close()
			return Progress{}, err
		}
		if _, err := out.Write(content); err != nil {
			out.Close()
			return Progress{}, err
		}
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return Progress{}, err
	}
	if err := out.Close(); err != nil {
		return Progress{}, err
	}
	if err := replaceFileAtomic(tmpPath, outPath); err != nil {
		return Progress{}, err
	}
	return Progress{Output: g.Output, FileCount: len(entries), Bytes: int64(offset)}, nil
}

func packageExcluded(group Group, packageName, path string) bool {
	for _, excludedPath := range group.ExcludeByPackage[packageName] {
		if normalize(excludedPath) == path {
			return true
		}
	}
	return false
}

func readOutputContent(entry sourceEntry, group Group) ([]byte, error) {
	content, err := readContent(entry)
	if err != nil {
		return nil, err
	}
	if group.SoundVolumePercent == nil || *group.SoundVolumePercent == 100 ||
		!strings.HasPrefix(entry.path, "sound/") || !strings.HasSuffix(entry.path, ".wav") {
		return content, nil
	}
	adjusted, _, err := ScaleWAVVolume(content, *group.SoundVolumePercent)
	if err != nil {
		return nil, err
	}
	return adjusted, nil
}

func ScaleWAVVolume(data []byte, percent int) ([]byte, bool, error) {
	if percent < 0 || percent > 100 {
		return nil, false, fmt.Errorf("volume percent must be 0-100")
	}
	if percent == 100 {
		return data, false, nil
	}
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return data, false, nil
	}
	var formatTag, bitsPerSample uint16
	var dataStart, dataSize int
	for offset := 12; offset+8 <= len(data); {
		chunkID := string(data[offset : offset+4])
		chunkSize := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		if chunkSize < 0 || offset+8+chunkSize > len(data) {
			return nil, false, fmt.Errorf("invalid WAV chunk")
		}
		chunkStart := offset + 8
		switch chunkID {
		case "fmt ":
			if chunkSize >= 16 {
				formatTag = binary.LittleEndian.Uint16(data[chunkStart : chunkStart+2])
				bitsPerSample = binary.LittleEndian.Uint16(data[chunkStart+14 : chunkStart+16])
			}
		case "data":
			dataStart = chunkStart
			dataSize = chunkSize
		}
		offset = chunkStart + chunkSize
		if chunkSize%2 == 1 {
			offset++
		}
	}
	if dataStart == 0 || dataSize == 0 || formatTag == 0 || bitsPerSample == 0 {
		return data, false, nil
	}
	factor := float64(percent) / 100
	out := append([]byte(nil), data...)
	samples := out[dataStart : dataStart+dataSize]
	switch formatTag {
	case 1:
		switch bitsPerSample {
		case 8:
			for i := range samples {
				centered := int(samples[i]) - 128
				samples[i] = byte(clampInt(int(math.Round(float64(centered)*factor))+128, 0, 255))
			}
		case 16:
			for i := 0; i+2 <= len(samples); i += 2 {
				value := int16(binary.LittleEndian.Uint16(samples[i : i+2]))
				binary.LittleEndian.PutUint16(samples[i:i+2], uint16(int16(clampInt(int(math.Round(float64(value)*factor)), -32768, 32767))))
			}
		case 24:
			for i := 0; i+3 <= len(samples); i += 3 {
				value := int(samples[i]) | int(samples[i+1])<<8 | int(samples[i+2])<<16
				if value&0x800000 != 0 {
					value |= ^0xffffff
				}
				scaled := clampInt(int(math.Round(float64(value)*factor)), -8388608, 8388607)
				samples[i] = byte(scaled)
				samples[i+1] = byte(scaled >> 8)
				samples[i+2] = byte(scaled >> 16)
			}
		case 32:
			for i := 0; i+4 <= len(samples); i += 4 {
				value := int32(binary.LittleEndian.Uint32(samples[i : i+4]))
				binary.LittleEndian.PutUint32(samples[i:i+4], uint32(int32(clampInt64(int64(math.Round(float64(value)*factor)), -2147483648, 2147483647))))
			}
		default:
			return data, false, nil
		}
	case 3:
		switch bitsPerSample {
		case 32:
			for i := 0; i+4 <= len(samples); i += 4 {
				value := math.Float32frombits(binary.LittleEndian.Uint32(samples[i : i+4]))
				binary.LittleEndian.PutUint32(samples[i:i+4], math.Float32bits(value*float32(factor)))
			}
		case 64:
			for i := 0; i+8 <= len(samples); i += 8 {
				value := math.Float64frombits(binary.LittleEndian.Uint64(samples[i : i+8]))
				binary.LittleEndian.PutUint64(samples[i:i+8], math.Float64bits(value*factor))
			}
		default:
			return data, false, nil
		}
	default:
		return data, false, nil
	}
	return out, true, nil
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func clampInt64(value, min, max int64) int64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func LoadPlan(path string) (Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Plan{}, err
	}
	return ParsePlan(data)
}

func ParsePlan(data []byte) (Plan, error) {
	var cfg Plan
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Plan{}, err
	}
	return cfg, nil
}

func Run(cfg Plan, callback func(Progress)) error {
	for index, group := range cfg.Groups {
		progress, err := writeGroup(cfg, group)
		if err != nil {
			return fmt.Errorf("%s: %w", group.Output, err)
		}
		progress.GroupIndex = index + 1
		progress.GroupCount = len(cfg.Groups)
		if callback != nil {
			callback(progress)
		}
	}
	return nil
}
