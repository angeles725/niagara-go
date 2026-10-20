// Package registry reads versions/registry.json the way niagara_help_lib
// does (versions.identify, versions.install_dir, course.course_dir) for the
// helpbuild builders that start from a data root. Of the helpbuild packages
// it imports only pyfs; beyond it, index, pycompat and pyjson.
package registry
