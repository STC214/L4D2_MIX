package vpkmerge

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestScaleWAVVolume16BitPCM(t *testing.T) {
	wav := testPCM16WAV([]int16{1000, -1000, 32767, -32768})
	adjusted, changed, err := ScaleWAVVolume(wav, 50)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected PCM WAV to be changed")
	}
	data := adjusted[len(adjusted)-8:]
	got := []int16{
		int16(binary.LittleEndian.Uint16(data[0:2])),
		int16(binary.LittleEndian.Uint16(data[2:4])),
		int16(binary.LittleEndian.Uint16(data[4:6])),
		int16(binary.LittleEndian.Uint16(data[6:8])),
	}
	want := []int16{500, -500, 16384, -16384}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sample %d = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestScaleWAVVolumeLeavesUnsupportedData(t *testing.T) {
	data := []byte("not a wav")
	adjusted, changed, err := ScaleWAVVolume(data, 42)
	if err != nil {
		t.Fatal(err)
	}
	if changed || string(adjusted) != string(data) {
		t.Fatal("unsupported data should remain unchanged")
	}
}

func TestWriteGroupUpdatesCRCForScaledSound(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.wav")
	if err := os.WriteFile(source, testPCM16WAV([]int16{1000}), 0644); err != nil {
		t.Fatal(err)
	}
	volume := 50
	_, err := writeGroup(Plan{Output: root}, Group{
		Output:             "scaled.vpk",
		Title:              "Scaled",
		Overlay:            map[string]string{"sound/weapons/test.wav": source},
		SoundVolumePercent: &volume,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(filepath.Join(root, "scaled.vpk")); err != nil {
		t.Fatalf("scaled VPK should verify with updated CRC: %v", err)
	}
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
