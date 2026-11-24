package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

// The resource types used, and the language of every resource (English, United States).
const (
	rtIcon        = 3
	rtGroupIcon   = 14
	rtVersion     = 16
	langEnglishUS = 0x0409
)

// A resource is one blob in the resource tree of the program.
type resource struct {
	typ, id, lang uint16
	data          []byte
}

// An iconFrame is one size of the icon.
type iconFrame struct {
	size int
	png  []byte       // the file as it was read
	img  *image.NRGBA // its pixels
}

// loadIconFrames reads bluelatch-N.png for every N up to 256 in dir, smallest
// first. An icon cannot be larger than 256 pixels, so larger files, which are
// for other uses, are left out.
func loadIconFrames(dir string) ([]iconFrame, error) {
	files, err := filepath.Glob(filepath.Join(dir, "bluelatch-*.png"))
	if err != nil {
		return nil, err
	}
	var frames []iconFrame
	for _, file := range files {
		name := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(file), "bluelatch-"), ".png")
		size, err := strconv.Atoi(name)
		if err != nil || size < 1 || size > 256 {
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		decoded, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		if b := decoded.Bounds(); b.Dx() != size || b.Dy() != size {
			return nil, fmt.Errorf("%s is %dx%d, its name says %d", file, b.Dx(), b.Dy(), size)
		}
		img := image.NewNRGBA(decoded.Bounds())
		draw.Draw(img, img.Bounds(), decoded, decoded.Bounds().Min, draw.Src)
		frames = append(frames, iconFrame{size: size, png: data, img: img})
	}
	if len(frames) == 0 {
		return nil, fmt.Errorf("no bluelatch-N.png files with N up to 256 in %s", dir)
	}
	sort.Slice(frames, func(i, j int) bool { return frames[i].size < frames[j].size })
	return frames, nil
}

// dib returns the pixels of an icon frame the way an icon resource stores them:
// a bitmap header whose height counts the colour bitmap and the mask, the pixels
// as bottom-up 32-bit BGRA with a straight alpha channel, and an all-zero mask
// (the alpha channel does the work).
func dib(img *image.NRGBA) []byte {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	maskStride := (w + 31) / 32 * 4
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, struct {
		Size                         uint32
		Width, Height                int32
		Planes, BitCount             uint16
		Compression, SizeImage       uint32
		XPelsPerMeter, YPelsPerMeter int32
		ColorsUsed, ColorsImportant  uint32
	}{Size: 40, Width: int32(w), Height: int32(2 * h), Planes: 1, BitCount: 32, SizeImage: uint32(w*h*4 + maskStride*h)})
	for y := h - 1; y >= 0; y-- {
		for x := 0; x < w; x++ {
			c := img.NRGBAAt(img.Bounds().Min.X+x, img.Bounds().Min.Y+y)
			b.Write([]byte{c.B, c.G, c.R, c.A})
		}
	}
	b.Write(make([]byte, maskStride*h))
	return b.Bytes()
}

// iconResources returns one RT_ICON resource for every frame, and the
// RT_GROUP_ICON resource that lists them. Frames of 256 pixels are stored as PNG
// files, smaller ones as bitmaps. The group has the ID 1, which is how the
// window finds its icon, and it is the only group, so it is also the icon that
// Explorer shows for the program.
func iconResources(frames []iconFrame) []resource {
	var out []resource
	var group bytes.Buffer
	binary.Write(&group, binary.LittleEndian, [3]uint16{0, 1, uint16(len(frames))})
	for i, f := range frames {
		data := dib(f.img)
		if f.size >= 256 {
			data = f.png
		}
		id := uint16(i + 1)
		out = append(out, resource{typ: rtIcon, id: id, lang: langEnglishUS, data: data})
		binary.Write(&group, binary.LittleEndian, struct {
			Width, Height, ColorCount, Reserved uint8
			Planes, BitCount                    uint16
			BytesInRes                          uint32
			ID                                  uint16
		}{Width: uint8(f.size), Height: uint8(f.size), Planes: 1, BitCount: 32, BytesInRes: uint32(len(data)), ID: id}) // 256 becomes 0, which means 256
	}
	return append(out, resource{typ: rtGroupIcon, id: 1, lang: langEnglishUS, data: group.Bytes()})
}

// versionInfo is what Explorer shows on the Details tab of the properties.
type versionInfo struct {
	Version          [4]uint16
	ProductName      string
	FileDescription  string
	OriginalFilename string
	InternalName     string
	CompanyName      string
	LegalCopyright   string
}

// parseVersion reads 1 to 4 numbers separated by dots, such as 0.1.2.
func parseVersion(s string) ([4]uint16, error) {
	var v [4]uint16
	parts := strings.Split(s, ".")
	if len(parts) > 4 {
		return v, fmt.Errorf("version %q has more than 4 parts", s)
	}
	for i, p := range parts {
		n, err := strconv.ParseUint(p, 10, 16)
		if err != nil {
			return v, fmt.Errorf("version %q: %w", s, err)
		}
		v[i] = uint16(n)
	}
	return v, nil
}

// A vnode is one structure of a version resource: a header, a key, a value, and
// children. Every structure starts on a 4-byte boundary.
type vnode struct {
	key      string
	text     bool   // the value is text, not binary
	valueLen uint16 // as stored: characters for a text value, bytes for a binary one
	value    []byte
	children []vnode
}

func pad4(b *bytes.Buffer) {
	for b.Len()%4 != 0 {
		b.WriteByte(0)
	}
}

func (n vnode) encode() []byte {
	var b bytes.Buffer
	put := func(v uint16) { binary.Write(&b, binary.LittleEndian, v) }
	put(0) // the length, filled in at the end
	put(n.valueLen)
	if n.text {
		put(1)
	} else {
		put(0)
	}
	for _, r := range utf16.Encode([]rune(n.key)) {
		put(r)
	}
	put(0)
	pad4(&b)
	b.Write(n.value)
	for _, c := range n.children {
		pad4(&b)
		b.Write(c.encode())
	}
	out := b.Bytes()
	binary.LittleEndian.PutUint16(out[:2], uint16(len(out)))
	return out
}

func utf16z(s string) []byte {
	var b bytes.Buffer
	for _, r := range append(utf16.Encode([]rune(s)), 0) {
		binary.Write(&b, binary.LittleEndian, r)
	}
	return b.Bytes()
}

// encodeVersion builds the VS_VERSIONINFO structure.
func encodeVersion(vi versionInfo) []byte {
	var fixed bytes.Buffer
	ms := uint32(vi.Version[0])<<16 | uint32(vi.Version[1])
	ls := uint32(vi.Version[2])<<16 | uint32(vi.Version[3])
	binary.Write(&fixed, binary.LittleEndian, [13]uint32{
		0xFEEF04BD, 0x00010000, // signature, structure version
		ms, ls, ms, ls, // file version, product version
		0x3F, 0, // flags mask, flags
		0x00040004, 1, 0, // Windows NT, application, no subtype
		0, 0, // no date
	})

	var strs []vnode
	add := func(key, value string) {
		if value != "" {
			strs = append(strs, vnode{key: key, text: true, valueLen: uint16(len(utf16.Encode([]rune(value))) + 1), value: utf16z(value)})
		}
	}
	version := fmt.Sprintf("%d.%d.%d", vi.Version[0], vi.Version[1], vi.Version[2])
	add("CompanyName", vi.CompanyName)
	add("FileDescription", vi.FileDescription)
	add("FileVersion", version)
	add("InternalName", vi.InternalName)
	add("LegalCopyright", vi.LegalCopyright)
	add("OriginalFilename", vi.OriginalFilename)
	add("ProductName", vi.ProductName)
	add("ProductVersion", version)

	translation := make([]byte, 4)
	binary.LittleEndian.PutUint32(translation, langEnglishUS|1200<<16) // English, Unicode
	root := vnode{
		key: "VS_VERSION_INFO", valueLen: 52, value: fixed.Bytes(),
		children: []vnode{
			{key: "StringFileInfo", text: true, children: []vnode{
				{key: "040904b0", text: true, children: strs},
			}},
			{key: "VarFileInfo", text: true, children: []vnode{
				{key: "Translation", valueLen: 4, value: translation},
			}},
		},
	}
	return root.encode()
}

// The machine types and the "address without image base" relocations of the
// 64-bit Windows architectures. The linker of Go turns such a relocation in a
// .rsrc section into the address of the resource data in the finished program.
var architectures = map[string]struct{ machine, relocation uint16 }{
	"amd64": {0x8664, 0x0003}, // IMAGE_FILE_MACHINE_AMD64, IMAGE_REL_AMD64_ADDR32NB
	"arm64": {0xAA64, 0x0002}, // IMAGE_FILE_MACHINE_ARM64, IMAGE_REL_ARM64_ADDR32NB
}

// layout builds the resource section: a tree of directories (type, then ID, then
// language) over data entries, followed by the data. It returns the section and
// the offsets of the fields that hold the address of a blob. Those fields are
// written as offsets within the section; the relocations turn them into
// addresses.
func layout(resources []resource) (section []byte, fixups []uint32, err error) {
	byType := map[uint16]map[uint16]map[uint16]*resource{}
	for i := range resources {
		r := &resources[i]
		if byType[r.typ] == nil {
			byType[r.typ] = map[uint16]map[uint16]*resource{}
		}
		if byType[r.typ][r.id] == nil {
			byType[r.typ][r.id] = map[uint16]*resource{}
		}
		if byType[r.typ][r.id][r.lang] != nil {
			return nil, nil, fmt.Errorf("resource type %d id %d language %#x twice", r.typ, r.id, r.lang)
		}
		byType[r.typ][r.id][r.lang] = r
	}
	keys := func(m map[uint16]map[uint16]*resource) []uint16 {
		var k []uint16
		for id := range m {
			k = append(k, id)
		}
		sort.Slice(k, func(i, j int) bool { return k[i] < k[j] })
		return k
	}
	var types []uint16
	for t := range byType {
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool { return types[i] < types[j] })

	const dirSize, entrySize, dataEntrySize = 16, 8, 16

	// Pass 1: where everything goes. The directories come first, then the data
	// entries, then the data.
	off := dirSize + entrySize*len(types)
	typeDir := map[uint16]int{}
	idDir := map[[2]uint16]int{}
	for _, t := range types {
		typeDir[t] = off
		off += dirSize + entrySize*len(byType[t])
	}
	for _, t := range types {
		for _, id := range keys(byType[t]) {
			idDir[[2]uint16{t, id}] = off
			off += dirSize + entrySize*len(byType[t][id])
		}
	}
	dataEntry := map[*resource]int{}
	var ordered []*resource
	for _, t := range types {
		for _, id := range keys(byType[t]) {
			var langs []uint16
			for l := range byType[t][id] {
				langs = append(langs, l)
			}
			sort.Slice(langs, func(i, j int) bool { return langs[i] < langs[j] })
			for _, l := range langs {
				r := byType[t][id][l]
				dataEntry[r] = off
				ordered = append(ordered, r)
				off += dataEntrySize
			}
		}
	}
	blob := map[*resource]int{}
	for _, r := range ordered {
		off = (off + 3) &^ 3
		blob[r] = off
		off += len(r.data)
	}
	section = make([]byte, (off+3)&^3)

	// Pass 2: write it.
	put16 := func(at int, v uint16) { binary.LittleEndian.PutUint16(section[at:], v) }
	put32 := func(at int, v uint32) { binary.LittleEndian.PutUint32(section[at:], v) }
	directory := func(at, entries int) { put16(at+14, uint16(entries)) } // entries by ID; none by name
	directory(0, len(types))
	for i, t := range types {
		e := entrySize*i + dirSize
		put32(e, uint32(t))
		put32(e+4, 0x80000000|uint32(typeDir[t]))

		d := typeDir[t]
		ids := keys(byType[t])
		directory(d, len(ids))
		for j, id := range ids {
			e := d + dirSize + entrySize*j
			put32(e, uint32(id))
			put32(e+4, 0x80000000|uint32(idDir[[2]uint16{t, id}]))

			dd := idDir[[2]uint16{t, id}]
			var langs []uint16
			for l := range byType[t][id] {
				langs = append(langs, l)
			}
			sort.Slice(langs, func(i, j int) bool { return langs[i] < langs[j] })
			directory(dd, len(langs))
			for k, l := range langs {
				r := byType[t][id][l]
				e := dd + dirSize + entrySize*k
				put32(e, uint32(l))
				put32(e+4, uint32(dataEntry[r]))

				de := dataEntry[r]
				put32(de, uint32(blob[r])) // an offset; the relocation makes it an address
				put32(de+4, uint32(len(r.data)))
				copy(section[blob[r]:], r.data)
				fixups = append(fixups, uint32(de))
			}
		}
	}
	return section, fixups, nil
}

// object builds a COFF object file with one .rsrc section, which is what a .syso
// file is. The relocations point at the symbol of the section itself.
func object(arch string, resources []resource) ([]byte, error) {
	a, ok := architectures[arch]
	if !ok {
		return nil, fmt.Errorf("unknown architecture %q (want amd64 or arm64)", arch)
	}
	section, fixups, err := layout(resources)
	if err != nil {
		return nil, err
	}

	const headerSize, sectionHeaderSize, relocationSize, symbolSize = 20, 40, 10, 18
	rawOffset := headerSize + sectionHeaderSize
	relocOffset := rawOffset + len(section)
	symbolOffset := relocOffset + relocationSize*len(fixups)

	var b bytes.Buffer
	w := func(v any) { binary.Write(&b, binary.LittleEndian, v) }
	// File header.
	w(a.machine)
	w(uint16(1)) // sections
	w(uint32(0)) // time stamp: zero, so the same input gives the same file
	w(uint32(symbolOffset))
	w(uint32(1)) // symbols
	w(uint16(0)) // optional header
	w(uint16(0)) // characteristics
	// Section header.
	b.Write([]byte{'.', 'r', 's', 'r', 'c', 0, 0, 0})
	w(uint32(0)) // virtual size
	w(uint32(0)) // virtual address
	w(uint32(len(section)))
	w(uint32(rawOffset))
	w(uint32(relocOffset))
	w(uint32(0)) // line numbers
	w(uint16(len(fixups)))
	w(uint16(0))
	w(uint32(0x40000040)) // initialized data, readable
	// Section data and relocations.
	b.Write(section)
	for _, at := range fixups {
		w(at)
		w(uint32(0)) // symbol 0: the section symbol
		w(a.relocation)
	}
	// The symbol of the section: static, named .rsrc, value 0, section 1.
	b.Write([]byte{'.', 'r', 's', 'r', 'c', 0, 0, 0})
	w(uint32(0))
	w(int16(1))
	w(uint16(0))
	w(uint8(3)) // IMAGE_SYM_CLASS_STATIC
	w(uint8(0))
	// The string table is empty: only its own size.
	w(uint32(4))
	return b.Bytes(), nil
}

// build makes the file for one program.
func build(arch, iconDir string, vi versionInfo) ([]byte, error) {
	frames, err := loadIconFrames(iconDir)
	if err != nil {
		return nil, err
	}
	resources := iconResources(frames)
	resources = append(resources, resource{typ: rtVersion, id: 1, lang: langEnglishUS, data: encodeVersion(vi)})
	if vi.ProductName == "" {
		return nil, errors.New("the product name is empty")
	}
	return object(arch, resources)
}
