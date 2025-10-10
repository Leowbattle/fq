package pyc

// pyc spec: https://github.com/python/cpython/blob/main/Python/marshal.c

import (
	"embed"
	"time"

	"github.com/wader/fq/format"
	"github.com/wader/fq/pkg/decode"
	"github.com/wader/fq/pkg/interp"
	"github.com/wader/fq/pkg/scalar"
	"golang.org/x/text/encoding/unicode"
)

var pycFS embed.FS

func init() {
	interp.RegisterFormat(
		format.PYC,
		&decode.Format{
			Description: "Python Bytecode",
			DecodeFn:    decodePYC,
			Functions:   []string{},
		})
	interp.RegisterFS(pycFS)
}

const (
	TYPE_NULL           = '0'
	TYPE_NONE           = 'N'
	TYPE_FALSE          = 'F'
	TYPE_TRUE           = 'T'
	TYPE_STOPITER       = 'S'
	TYPE_ELLIPSIS       = '.'
	TYPE_BINARY_FLOAT   = 'g' // Version 0 uses TYPE_FLOAT instead.
	TYPE_BINARY_COMPLEX = 'y' // Version 0 uses TYPE_COMPLEX instead.
	TYPE_LONG           = 'l' // See also TYPE_INT.
	TYPE_STRING         = 's' // Bytes. (Name comes from Python 2.)
	TYPE_TUPLE          = '(' // See also TYPE_SMALL_TUPLE.
	TYPE_LIST           = '['
	TYPE_DICT           = '{'
	TYPE_CODE           = 'c'
	TYPE_UNICODE        = 'u'
	TYPE_UNKNOWN        = '?'
	TYPE_SET            = '<' // added in version 2
	TYPE_FROZENSET      = '>' // added in version 2
	TYPE_SLICE          = ':' // added in version 5

	// Special cases for unicode strings (added in version 4)
	TYPE_INTERNED             = 't' // Version 1+
	TYPE_ASCII                = 'a'
	TYPE_ASCII_INTERNED       = 'A'
	TYPE_SHORT_ASCII          = 'z'
	TYPE_SHORT_ASCII_INTERNED = 'Z'

	// Special cases for small objects
	TYPE_INT         = 'i' // All versions. 32-bit encoding.
	TYPE_SMALL_TUPLE = ')' // Version 4+

	// Supported for backwards compatibility
	TYPE_COMPLEX = 'x' // Generated for version 0 only.
	TYPE_FLOAT   = 'f' // Generated for version 0 only.
	TYPE_INT64   = 'I' // Not generated any more.

	// References (added in version 3)
	TYPE_REF = 'r'
)

const FLAG_REF uint64 = 0x80 // with a type, add obj to index

var typeMap = scalar.UintMap{
	TYPE_NULL:           {Sym: "null", Description: "Not a Python object"},
	TYPE_NONE:           {Sym: "None", Description: "Python None"},
	TYPE_FALSE:          {Sym: "False", Description: "Python False"},
	TYPE_TRUE:           {Sym: "True", Description: "Python True"},
	TYPE_STOPITER:       {Sym: "StopIteration", Description: "StopIteration exception"},
	TYPE_ELLIPSIS:       {Sym: "Ellipsis", Description: "Ellipsis object"},
	TYPE_BINARY_FLOAT:   {Sym: "binary_float", Description: "Binary float"},
	TYPE_BINARY_COMPLEX: {Sym: "binary_complex", Description: "Binary complex"},
	TYPE_LONG:           {Sym: "long", Description: "Long integer (arbitrary precision)"},
	TYPE_STRING:         {Sym: "string", Description: "Byte string"},
	TYPE_TUPLE:          {Sym: "tuple", Description: "Tuple"},
	TYPE_LIST:           {Sym: "list", Description: "List"},
	TYPE_DICT:           {Sym: "dict", Description: "Dictionary"},
	TYPE_CODE:           {Sym: "code", Description: "Code object"},
	TYPE_UNICODE:        {Sym: "unicode", Description: "Unicode string"},
	TYPE_UNKNOWN:        {Sym: "unknown", Description: "Unknown type"},
	TYPE_SET:            {Sym: "set", Description: "Set"},
	TYPE_FROZENSET:      {Sym: "frozenset", Description: "Frozen set"},
	TYPE_SLICE:          {Sym: "slice", Description: "Slice object"},

	TYPE_INTERNED:             {Sym: "interned", Description: "Interned unicode string"},
	TYPE_ASCII:                {Sym: "ascii", Description: "ASCII unicode string"},
	TYPE_ASCII_INTERNED:       {Sym: "ascii_interned", Description: "Interned ASCII unicode string"},
	TYPE_SHORT_ASCII:          {Sym: "short_ascii", Description: "Short ASCII unicode string"},
	TYPE_SHORT_ASCII_INTERNED: {Sym: "short_ascii_interned", Description: "Interned short ASCII unicode string"},

	TYPE_INT:         {Sym: "int", Description: "Integer"},
	TYPE_SMALL_TUPLE: {Sym: "small_tuple", Description: "Small tuple"},

	TYPE_COMPLEX: {Sym: "complex", Description: "Complex number"},
	TYPE_FLOAT:   {Sym: "float", Description: "Float number"},
	TYPE_INT64:   {Sym: "int64", Description: "64-bit integer"},

	TYPE_REF: {Sym: "ref", Description: "Reference to an earlier object"},
}

func read_list(d *decode.D, n int64) {
	d.FieldStructNArray("items", "item", n, func(d *decode.D) {
		r_object(d)
	})
}

func r_object(d *decode.D) uint64 {
	ty := d.FieldUintFn("type", func(d *decode.D) uint64 {
		code := d.U8()
		return code & ^FLAG_REF
	}, typeMap)

	switch ty {
	// NO DATA
	case TYPE_NULL:
	case TYPE_NONE:
	case TYPE_STOPITER:
	case TYPE_ELLIPSIS:
	case TYPE_FALSE:
	case TYPE_TRUE:
	// NO DATA

	case TYPE_INT:
		d.FieldS32("value")
	case TYPE_INT64:
		d.FieldS64("value")
	case TYPE_LONG:
		panic("long not implemented")

	case TYPE_FLOAT:
		panic("float not implemented")
		// Seems to not be used any more?
		// d.TryFieldAnyFn("value", func(d *decode.D) (any, error) {
		// 	s := d.UTF8ShortString()
		// 	return strconv.ParseFloat(s, 64)
		// })

	case TYPE_BINARY_FLOAT:
		d.FieldF64("value")

	case TYPE_COMPLEX:
		panic("complex not implemented")

	case TYPE_BINARY_COMPLEX:
		d.FieldF64("real")
		d.FieldF64("imag")

	case TYPE_STRING:
		length := d.FieldS32("length")
		d.FieldRawLen("value", length*8)

	case TYPE_ASCII_INTERNED:
		fallthrough
	case TYPE_ASCII:
		length := d.FieldS32("length")
		d.FieldStr("value", int(length), unicode.UTF8)

	case TYPE_SHORT_ASCII_INTERNED:
		d.FieldUTF8ShortString("string")
	case TYPE_SHORT_ASCII:
		d.FieldUTF8ShortString("string")

	case TYPE_INTERNED:
		fallthrough
	case TYPE_UNICODE:
		length := d.FieldS32("length")
		d.FieldStr("value", int(length), unicode.UTF8)

	case TYPE_SMALL_TUPLE:
		n := d.FieldU8("n")
		read_list(d, int64(n))
	case TYPE_TUPLE:
		n := d.FieldS32("n")
		read_list(d, int64(n))

	case TYPE_LIST:
		n := d.FieldS32("n")
		read_list(d, int64(n))

	case TYPE_DICT:
		d.FieldArray("items", func(d *decode.D) {
			end := false
			for !end {
				d.FieldStruct("key", func(d *decode.D) {
					ty := r_object(d)
					end = ty == TYPE_NULL
				})
				if end {
					break
				}
				d.FieldStruct("value", func(d *decode.D) {
					ty := r_object(d)
					end = ty == TYPE_NULL
				})
			}
		})

	case TYPE_SET:
		fallthrough
	case TYPE_FROZENSET:
		n := d.FieldS32("n")
		read_list(d, int64(n))

	case TYPE_CODE:
		d.FieldS32("argcount")
		d.FieldS32("posonlyargcount")
		d.FieldS32("kwonlyargcount")
		d.FieldS32("stacksize")
		d.FieldS32("flags")
		d.FieldStruct("code", func(d *decode.D) { r_object(d) })
		d.FieldStruct("consts", func(d *decode.D) { r_object(d) })
		d.FieldStruct("names", func(d *decode.D) { r_object(d) })
		d.FieldStruct("localsplusnames", func(d *decode.D) { r_object(d) })
		d.FieldStruct("localspluskinds", func(d *decode.D) { r_object(d) })
		d.FieldStruct("filename", func(d *decode.D) { r_object(d) })
		d.FieldStruct("name", func(d *decode.D) { r_object(d) })
		d.FieldStruct("qualname", func(d *decode.D) { r_object(d) })
		d.FieldU32("firstlineno")
		d.FieldStruct("linetable", func(d *decode.D) { r_object(d) })
		d.FieldStruct("exceptiontable", func(d *decode.D) { r_object(d) })

	case TYPE_REF:
		d.FieldU32("index")

	case TYPE_SLICE:
		panic("slice not implemented")
	}

	return ty
}

func decodePYC(d *decode.D) any {
	d.Endian = decode.LittleEndian

	d.FieldU32("magic", scalar.UintHex)
	d.FieldRawLen("bit field", 4*8)
	d.FieldU32("timestamp", scalar.UintActualUnixTimeDescription(time.Second, time.RFC3339))
	d.FieldU32("length")

	d.FieldStruct("object", func(d *decode.D) { r_object(d) })

	return nil
}
