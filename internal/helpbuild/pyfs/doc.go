// Package pyfs holds the file-system and JSON helpers the helpbuild builders
// share, with the semantics of the Python calls they port: os.remove,
// shutil.copyfile and json.load (pyfs.go) (os.walk is pycompat.Walk),
// and the atomic JSON index write with its json.dumps option sets
// (jsonwrite.go). It imports no other helpbuild package.
package pyfs
