package qnx6read

import (
	"os"
	"sort"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/cli"
)

// doc is the --help description (RawDescriptionHelpFormatter), from the Python docstring.
const doc = "qnx6read: minimal read-only QNX6 (Power-Safe) reader for the JACE-8000 SD image (B674).\n" +
	"\n" +
	"Lists the directory tree of one partition of a raw disk image, or extracts one file. READ-ONLY.\n" +
	"\n" +
	"Usage:\n" +
	"  qnx6read [--image disk.img] [P2|P3]                  list the tree (default P2)\n" +
	"  qnx6read [--image disk.img] P2 extract <src> <dst>  copy one file out of the image\n" +
	"The image comes from --image, else from the QNX6_IMG environment variable: a regular file or a block\n" +
	"device (`--image /dev/sdb` reads the card directly); a character device (a BSD raw disk /dev/rdiskN)\n" +
	"only with --char-device, so a terminal or /dev/null is never read as a disk.\n" +
	"Exit 0 ok, 1 listing incomplete (depth limit hit), 2 usage (no image, an image that is not a file or a\n" +
	"block device, an unreadable image, bad arguments).\n" +
	""

// Main is the qnx6read command line (Python argparse parity). --image defaults to $QNX6_IMG.
func Main(args []string, s cli.Stdio) int {
	p := argparse.New(argparse.Config{Prog: "qnx6read", Description: doc,
		Formatter: argparse.RawDescriptionHelpFormatter})
	p.Add(argparse.Arg{Default: os.Getenv("QNX6_IMG"), Help: "raw disk image (default: $QNX6_IMG)"}, "--image")
	parts := make([]string, 0, len(Partitions))
	for k := range Partitions {
		parts = append(parts, k)
	}
	sort.Strings(parts)
	choices := make([]any, len(parts))
	for i, k := range parts {
		choices[i] = k
	}
	p.Add(argparse.Arg{Nargs: argparse.Optional, Default: "P2", Choices: choices, Help: "partition (default P2)"}, "part")
	p.Add(argparse.Arg{Nargs: argparse.ZeroOrMore, Metavar: "extract SRC DST",
		Help: "extract one file instead of listing"}, "action")
	p.Add(argparse.Arg{Action: argparse.StoreTrue,
		Help: "accept a character device as the image (a BSD raw disk); refused by default"}, "--char-device")
	ns, code, done := p.Parse(args, s)
	if done {
		return code
	}
	action := ns.Strings("action")
	if len(action) > 0 && (len(action) != 3 || action[0] != "extract") {
		return p.Error(s.Err, "expected: extract <src> <dst>")
	}
	o := Options{Image: ns.String("image"), Part: ns.String("part")}
	if msg := ValidateImage(o.Image, ns.Bool("char_device")); msg != "" {
		return p.Error(s.Err, msg)
	}
	if len(action) == 3 {
		o.Extract = &Extract{Src: action[1], Dst: action[2]}
	}
	return Run(o, s)
}
