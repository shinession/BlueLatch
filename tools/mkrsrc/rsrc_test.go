package main

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"
)

// writeIcons writes frames whose top row is red and whose bottom row is blue,
// with a transparent pixel at the top left corner, and some files that must be ignored.
func writeIcons(t *testing.T, sizes ...int) string {
	t.Helper()
	dir := t.TempDir()
	for _, n := range append(sizes, 512) { // 512 is too large for an icon
		img := image.NewNRGBA(image.Rect(0, 0, n, n))
		for y := 0; y < n; y++ {
			c := color.NRGBA{R: 255, A: 255}
			if y == n-1 {
				c = color.NRGBA{B: 255, A: 255}
			}
			for x := 0; x < n; x++ {
				img.SetNRGBA(x, y, c)
			}
		}
		img.SetNRGBA(0, 0, color.NRGBA{})
		var b bytes.Buffer
		if err := png.Encode(&b, img); err != nil {
			t.Fatal(err)
		}
		name := filepath.Join(dir, "bluelatch-"+strconv.Itoa(n)+".png")
		if err := os.WriteFile(name, b.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not an icon"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

var testVersion = versionInfo{
	Version:          [4]uint16{0, 1, 2, 0},
	ProductName:      "BlueLatch",
	FileDescription:  "BlueLatch test",
	OriginalFilename: "bluelatch.exe",
	InternalName:     "bluelatch",
}

func le16(b []byte) int { return int(binary.LittleEndian.Uint16(b)) }
func le32(b []byte) int { return int(binary.LittleEndian.Uint32(b)) }

// leaf is a resource found by walking the tree of a section.
type leaf struct {
	typ, id, lang int
	data          []byte
}

// walk reads the resource tree of the section. The data entries hold offsets
// within the section (the relocations are not applied in an object file), so the
// blobs can be read straight from it.
func walk(t *testing.T, section []byte) []leaf {
	t.Helper()
	var leaves []leaf
	var visit func(at int, path []int)
	visit = func(at int, path []int) {
		if le16(section[at+12:]) != 0 {
			t.Errorf("directory at %d has entries by name; there should be none", at)
		}
		n := le16(section[at+14:])
		prev := -1
		for i := 0; i < n; i++ {
			e := at + 16 + 8*i
			name, off := le32(section[e:]), le32(section[e+4:])
			if name <= prev {
				t.Errorf("directory at %d: IDs are not in ascending order (%d after %d)", at, name, prev)
			}
			prev = name
			if off&0x80000000 != 0 {
				visit(off&0x7fffffff, append(append([]int(nil), path...), name))
				continue
			}
			blob, size := le32(section[off:]), le32(section[off+4:])
			if blob%4 != 0 {
				t.Errorf("blob at %d is not aligned to 4 bytes", blob)
			}
			if len(path) != 2 {
				t.Errorf("a data entry at depth %d, want 3 levels (type, ID, language)", len(path)+1)
				continue
			}
			leaves = append(leaves, leaf{typ: path[0], id: path[1], lang: name, data: section[blob : blob+size]})
		}
	}
	visit(0, nil)
	return leaves
}

func find(t *testing.T, leaves []leaf, typ, id int) leaf {
	t.Helper()
	for _, l := range leaves {
		if l.typ == typ && l.id == id {
			return l
		}
	}
	t.Fatalf("no resource of type %d with ID %d", typ, id)
	return leaf{}
}

func TestObjectFile(t *testing.T) {
	dir := writeIcons(t, 16, 32, 256)
	data, err := build("amd64", dir, testVersion)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	f, err := pe.NewFile(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("debug/pe cannot read the object: %v", err)
	}
	if f.Machine != 0x8664 || len(f.Sections) != 1 || f.Sections[0].Name != ".rsrc" {
		t.Fatalf("machine %#x, sections %d: want amd64 and one .rsrc section", f.Machine, len(f.Sections))
	}
	s := f.Sections[0]
	if s.Characteristics&0x40 == 0 || s.Characteristics&0x40000000 == 0 {
		t.Errorf("section characteristics %#x lack initialized data or read", s.Characteristics)
	}
	section, err := s.Data()
	if err != nil {
		t.Fatal(err)
	}
	leaves := walk(t, section)
	if len(leaves) != 5 { // three icons, the group, the version
		t.Fatalf("%d resources, want 5", len(leaves))
	}
	if int(s.NumberOfRelocations) != len(leaves) || len(s.Relocs) != len(leaves) {
		t.Errorf("%d relocations for %d resources", s.NumberOfRelocations, len(leaves))
	}
	for _, r := range s.Relocs {
		if r.Type != 0x0003 || r.SymbolTableIndex != 0 {
			t.Errorf("relocation %+v, want ADDR32NB against the section symbol", r)
		}
	}
	if len(f.COFFSymbols) != 1 || f.COFFSymbols[0].StorageClass != 3 || f.COFFSymbols[0].SectionNumber != 1 || f.COFFSymbols[0].Name[0] != '.' {
		t.Errorf("symbols = %+v, want one static symbol named .rsrc in section 1", f.COFFSymbols)
	}
	for _, l := range leaves {
		if l.lang != 0x0409 {
			t.Errorf("resource %d/%d has language %#x, want 0x409", l.typ, l.id, l.lang)
		}
	}
}

func TestIcons(t *testing.T) {
	dir := writeIcons(t, 32, 16, 256) // out of order on purpose
	data, err := build("amd64", dir, testVersion)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := pe.NewFile(bytes.NewReader(data))
	section, _ := f.Sections[0].Data()
	leaves := walk(t, section)

	group := find(t, leaves, rtGroupIcon, 1).data
	if le16(group[0:]) != 0 || le16(group[2:]) != 1 || le16(group[4:]) != 3 {
		t.Fatalf("group header % x, want reserved 0, type 1, three images", group[:6])
	}
	sizes := []int{16, 32, 0} // 0 means 256, and the sizes ascend
	for i, want := range sizes {
		e := group[6+14*i:]
		w, h, planes, bits, bytesInRes, id := int(e[0]), int(e[1]), le16(e[4:]), le16(e[6:]), le32(e[8:]), le16(e[12:])
		if w != want || h != want || planes != 1 || bits != 32 {
			t.Errorf("entry %d: %dx%d, %d planes, %d bits; want %dx%d, 1, 32", i, w, h, planes, bits, want, want)
		}
		blob := find(t, leaves, rtIcon, id).data
		if bytesInRes != len(blob) {
			t.Errorf("entry %d says %d bytes, the icon resource has %d", i, bytesInRes, len(blob))
		}
	}

	// A small frame is a bitmap: header, bottom-up BGRA pixels, an empty mask.
	small := find(t, leaves, rtIcon, 1).data
	if le32(small[0:]) != 40 || le32(small[4:]) != 16 || le32(small[8:]) != 32 || le16(small[12:]) != 1 || le16(small[14:]) != 32 || le32(small[16:]) != 0 {
		t.Errorf("bitmap header % x", small[:40])
	}
	pixels := small[40:]
	if got := pixels[:4]; !bytes.Equal(got, []byte{255, 0, 0, 255}) {
		t.Errorf("first stored pixel % x, want blue (BGRA 255 0 0 255): rows are stored bottom-up", got)
	}
	lastRow := pixels[15*16*4:]
	if got := lastRow[4:8]; !bytes.Equal(got, []byte{0, 0, 255, 255}) {
		t.Errorf("a pixel of the last stored row % x, want red (BGRA 0 0 255 255): that is the top row", got)
	}
	if got := lastRow[:4]; !bytes.Equal(got, []byte{0, 0, 0, 0}) {
		t.Errorf("the transparent corner is % x, want 0 0 0 0", got)
	}
	if mask := small[40+16*16*4:]; len(mask) != 4*16 || !bytes.Equal(mask, make([]byte, 4*16)) {
		t.Errorf("mask is %d bytes (want 64) or not empty", len(mask))
	}

	// The frame of 256 pixels is the PNG file itself.
	if big := find(t, leaves, rtIcon, 3).data; !bytes.HasPrefix(big, []byte("\x89PNG\r\n\x1a\n")) {
		t.Errorf("the 256 pixel frame does not start with a PNG signature: % x", big[:8])
	}
}

// pnode is a structure of a version resource, read back.
type pnode struct {
	key      string
	text     bool
	value    []byte
	children []pnode
}

func parseNode(t *testing.T, b []byte) (pnode, int) {
	t.Helper()
	length, vlen, typ := le16(b), le16(b[2:]), le16(b[4:])
	if length > len(b) {
		t.Fatalf("a structure says it is %d bytes, only %d are left", length, len(b))
	}
	i := 6
	var key []uint16
	for {
		c := uint16(le16(b[i:]))
		i += 2
		if c == 0 {
			break
		}
		key = append(key, c)
	}
	i = (i + 3) &^ 3
	size := vlen
	if typ == 1 {
		size = vlen * 2
	}
	n := pnode{key: string(utf16.Decode(key)), text: typ == 1, value: b[i : i+size]}
	i += size
	for i < length {
		i = (i + 3) &^ 3
		child, used := parseNode(t, b[i:length])
		n.children = append(n.children, child)
		i += used
	}
	return n, length
}

func TestVersionInfo(t *testing.T) {
	dir := writeIcons(t, 16)
	data, err := build("amd64", dir, testVersion)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := pe.NewFile(bytes.NewReader(data))
	section, _ := f.Sections[0].Data()
	blob := find(t, walk(t, section), rtVersion, 1).data

	root, used := parseNode(t, blob)
	if used != len(blob) {
		t.Errorf("the structure is %d bytes, the resource %d", used, len(blob))
	}
	if root.key != "VS_VERSION_INFO" || root.text || len(root.value) != 52 {
		t.Fatalf("root = %q, text %v, %d value bytes", root.key, root.text, len(root.value))
	}
	fixed := root.value
	if le32(fixed[0:]) != 0xFEEF04BD || le32(fixed[4:]) != 0x00010000 {
		t.Errorf("fixed info signature % x", fixed[:8])
	}
	if le32(fixed[8:]) != 0x00000001 || le32(fixed[12:]) != 0x00020000 || le32(fixed[16:]) != 0x00000001 || le32(fixed[20:]) != 0x00020000 {
		t.Errorf("fixed info versions % x, want 0.1.2.0 for the file and the product", fixed[8:24])
	}

	if len(root.children) != 2 || root.children[0].key != "StringFileInfo" || root.children[1].key != "VarFileInfo" {
		t.Fatalf("children of the root: %+v", root.children)
	}
	table := root.children[0].children
	if len(table) != 1 || table[0].key != "040904b0" {
		t.Fatalf("string tables: %+v", table)
	}
	got := map[string]string{}
	for _, s := range table[0].children {
		if !s.text || len(s.value) < 2 || s.value[len(s.value)-1] != 0 || s.value[len(s.value)-2] != 0 {
			t.Errorf("string %q is not text ending in a NUL", s.key)
			continue
		}
		var u []uint16
		for i := 0; i+1 < len(s.value)-2; i += 2 {
			u = append(u, uint16(le16(s.value[i:])))
		}
		got[s.key] = string(utf16.Decode(u))
	}
	want := map[string]string{
		"FileDescription":  "BlueLatch test",
		"FileVersion":      "0.1.2",
		"InternalName":     "bluelatch",
		"OriginalFilename": "bluelatch.exe",
		"ProductName":      "BlueLatch",
		"ProductVersion":   "0.1.2",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("strings = %v, want %v (empty ones are left out)", got, want)
	}
	vars := root.children[1].children
	if len(vars) != 1 || vars[0].key != "Translation" || le32(vars[0].value) != 0x04B00409 {
		t.Errorf("var info = %+v, want Translation 0x04B00409", vars)
	}
}

func TestArm64Object(t *testing.T) {
	data, err := build("arm64", writeIcons(t, 16), testVersion)
	if err != nil {
		t.Fatal(err)
	}
	f, err := pe.NewFile(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if f.Machine != 0xAA64 {
		t.Errorf("machine %#x, want 0xAA64", f.Machine)
	}
	for _, r := range f.Sections[0].Relocs {
		if r.Type != 0x0002 {
			t.Errorf("relocation type %d, want ARM64 ADDR32NB (2)", r.Type)
		}
	}
}

func TestSameInputSameFile(t *testing.T) {
	dir := writeIcons(t, 16, 32)
	a, _ := build("amd64", dir, testVersion)
	b, _ := build("amd64", dir, testVersion)
	if !bytes.Equal(a, b) {
		t.Error("two builds of the same input differ")
	}
}

func TestSizesAreSorted(t *testing.T) {
	frames, err := loadIconFrames(writeIcons(t, 256, 16, 48, 32))
	if err != nil {
		t.Fatal(err)
	}
	var sizes []int
	for _, f := range frames {
		sizes = append(sizes, f.size)
	}
	if !sort.IntsAreSorted(sizes) || len(sizes) != 4 {
		t.Errorf("sizes = %v, want 16 32 48 256 (512 and the text file are left out)", sizes)
	}
}

func TestProblems(t *testing.T) {
	good := writeIcons(t, 16)
	if _, err := build("riscv", good, testVersion); err == nil {
		t.Error("an unknown architecture was accepted")
	}
	if _, err := build("amd64", t.TempDir(), testVersion); err == nil {
		t.Error("an empty icon directory was accepted")
	}
	noName := testVersion
	noName.ProductName = ""
	if _, err := build("amd64", good, noName); err == nil {
		t.Error("an empty product name was accepted")
	}

	// A file whose name says 32 but which is 16 pixels wide.
	dir := t.TempDir()
	src, _ := os.ReadFile(filepath.Join(good, "bluelatch-16.png"))
	if err := os.WriteFile(filepath.Join(dir, "bluelatch-32.png"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := build("amd64", dir, testVersion); err == nil || !strings.Contains(err.Error(), "its name says 32") {
		t.Errorf("a frame of the wrong size: error = %v", err)
	}

	for _, v := range []string{"", "1.2.3.4.5", "1.x", "70000"} {
		if _, err := parseVersion(v); err == nil {
			t.Errorf("parseVersion(%q) accepted it", v)
		}
	}
	if v, err := parseVersion("0.1.2"); err != nil || v != [4]uint16{0, 1, 2, 0} {
		t.Errorf("parseVersion(0.1.2) = %v, %v", v, err)
	}
}

func TestTwoResourcesWithTheSameName(t *testing.T) {
	_, _, err := layout([]resource{{typ: 3, id: 1, lang: 0x409}, {typ: 3, id: 1, lang: 0x409}})
	if err == nil {
		t.Error("two resources with the same type, ID and language were accepted")
	}
}
