# generate-builder-opts

I love functional options in go. I think it's a great pattern to build
future-proof APIs.

Sometimes, it can be a little tedious to define the functional option function 
type and functions for all the fields of your type. It's also a little repetitive
to have to keep the set of functional option functions in sync with the list of
struct fields.

Not anymore.

If your structs' fields have types that are (1) identifiers and (2) either
global or defined in-package, just generate code for them!

## Usage

```shell script
$ go get github.com/splunk/go-generate-builder-opts/cmd/generate-builder-opts
$ generate-builder-opts
Usage:
  -definitionFile string
    	file where type is defined (required)
  -exportOptionFuncType
    	whether to export the configuration function type (optional) (default true)
  -generateForUnexportedFields
    	whether to generate configuration functions for unexported fields (optional)
  -ignoreUnsupported
    	whether to skip fields whose type we can't handle (error otherwise) (optional) (default true)
  -outFile string
    	file to write builder option functions to; stdout if omitted (optional)
  -skipStructFields value
    	comma-separated list of struct fields to ignore (exact match)
  -structTypeName string
    	fieldName of type to generate builder options for (required)
```

### Example

```shell script
$ cat struct_def.go
package genbuildertest

type A struct {
    B string
    C int
    D bool
    e float32
    F interface{}
}

$ ./generate-builder-opts -definitionFile struct_def.go -structTypeName A -outFile gend_functional_opts.go
$ cat gend_functional_opts.go
package genbuildertest

type AFieldSetter func(*A)

func SetB(bGen string) AFieldSetter {
	return func(aGen *A) {
		aGen.B = bGen
	}
}
func SetC(cGen int) AFieldSetter {
	return func(aGen *A) {
		aGen.C = cGen
	}
}
func SetD(dGen bool) AFieldSetter {
	return func(aGen *A) {
		aGen.D = dGen
	}
}
func SetF(fGen interface{}) AFieldSetter {
	return func(aGen *A) {
		aGen.F = fGen
	}
}

```

### Limitations

#### Imported Types

The use of the `go/ast` package makes this pretty powerful with pretty limited
effort, however imported types seem to be difficult to deal with (perhaps
impossible w/out typechecking?) so this does not handle fields whose type is
from another package. Use the `-ignoreUnsupported` flag to tell the program not
to error and exit when one of these is encountered.

#### Embedded Fields

I have made the conscious choice to not handle embedded fields. We could in the
future, but I'm not sure how it ought to behave.

#### Formatting

The `go/printer` package seems to output functions with no empty line between
them. I'm not really sure what's going on here, but it's easy enough to run the
output through `go fmt`.

#### Comments

It sure would be nice to generate a comment to the effect of "This function is
a setter for field \<field name\>", but comments seem to be very difficult to work
with without playing with actual token positions, so maybe something to do in
the future.