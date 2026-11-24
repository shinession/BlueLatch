// Command mkrsrc writes a .syso file that gives a Windows program its icon and the
// version information that Explorer shows in the properties of the file. The Go
// toolchain links a .syso file that sits in the directory of a main package, so
// the resources are part of every build, with no tool needed to build.
//
// It uses the standard library only. The icon is made of the files
// bluelatch-N.png (N up to 256) in the directory of the icon frames. There is no
// manifest: the programs set what a manifest would at run time.
//
// Run scripts\resources.ps1 rather than this command; it makes the files for both
// architectures.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	arch := flag.String("arch", "amd64", "architecture of the program: amd64 or arm64")
	out := flag.String("out", "", "the .syso file to write, such as cmd/bluelatch/rsrc_windows_amd64.syso")
	icons := flag.String("icons", "assets/logo/png", "directory with the icon frames bluelatch-N.png")
	version := flag.String("version", "", "version of the program, 1 to 4 numbers separated by dots, such as 0.1.2")
	product := flag.String("product-name", "BlueLatch", "product name")
	description := flag.String("description", "", "file description")
	original := flag.String("original-filename", "", "original file name, such as bluelatch.exe")
	internal := flag.String("internal-name", "", "internal name, such as bluelatch")
	company := flag.String("company", "", "company name (left out when empty)")
	copyright := flag.String("copyright", "", "legal copyright (left out when empty)")
	flag.Parse()

	if err := run(*arch, *out, *icons, *version, versionInfo{
		ProductName:      *product,
		FileDescription:  *description,
		OriginalFilename: *original,
		InternalName:     *internal,
		CompanyName:      *company,
		LegalCopyright:   *copyright,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "mkrsrc:", err)
		os.Exit(1)
	}
}

func run(arch, out, icons, version string, vi versionInfo) error {
	if out == "" {
		return fmt.Errorf("-out is required")
	}
	v, err := parseVersion(version)
	if err != nil {
		return err
	}
	vi.Version = v
	data, err := build(arch, icons, vi)
	if err != nil {
		return err
	}
	return os.WriteFile(out, data, 0o644)
}
