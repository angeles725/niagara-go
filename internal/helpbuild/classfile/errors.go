package classfile

import "fmt"

// BadMagicError is a file whose first four bytes are not 0xCAFEBABE.
type BadMagicError struct{ Got uint32 }

func (e *BadMagicError) Error() string {
	return fmt.Sprintf("classfile: bad magic 0x%08X, want 0xCAFEBABE", e.Got)
}

// TruncatedError is a file that ends inside a structure: What starting at
// Offset needed Need more bytes than were left.
type TruncatedError struct {
	Offset, Need int
	What         string
}

func (e *TruncatedError) Error() string {
	return fmt.Sprintf("classfile: truncated at offset %d: %s needs %d more bytes", e.Offset, e.What, e.Need)
}

// UnknownTagError is a constant pool entry whose tag no class file version
// through Java 21 defines.
type UnknownTagError struct {
	Index, Offset int
	Tag           uint8
}

func (e *UnknownTagError) Error() string {
	return fmt.Sprintf("classfile: unknown constant pool tag %d at index %d (offset %d)", e.Tag, e.Index, e.Offset)
}

// IndexError is a constant pool reference that is out of range, lands on an
// unusable slot (Got 0) or on an entry of another kind than Want.
type IndexError struct {
	Index, Size int
	Want, Got   Tag
}

func (e *IndexError) Error() string {
	if e.Index <= 0 || e.Index >= e.Size {
		return fmt.Sprintf("classfile: constant pool index %d out of range 1..%d, want %v", e.Index, e.Size-1, e.Want)
	}
	return fmt.Sprintf("classfile: constant pool index %d is %v, want %v", e.Index, e.Got, e.Want)
}

// FormatError is any other structural fault, found at Offset.
type FormatError struct {
	Offset int
	Msg    string
}

func (e *FormatError) Error() string {
	return fmt.Sprintf("classfile: offset %d: %s", e.Offset, e.Msg)
}
