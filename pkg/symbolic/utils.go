package symbolic

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"
)

// #include "../../symbolic-cabi/include/symbolic.h"
import "C"

// decodes a string from Symbolic into a Go string, and frees the memory of the original string
// this can be safely called on strings that are "owned by Symbolic", because freeing them won't actually do anything
func decodeAndFreeString(cStr C.SymbolicStr) string {
	str := C.GoStringN(cStr.data, C.int(cStr.len))
	// we don't need error handling here, so no need to use wrapCall
	C.symbolic_str_free(&cStr)
	return str
}

// encodes a string for passing to Symbolic. This allocates memory in the C heap.
// It's the caller's responsibility to call free() to free this memory when the string is not used anymore
func encodeString(str string) (symStr C.SymbolicStr, free func()) {
	cstr := C.CString(str)
	symStr = C.SymbolicStr{
		data:  cstr,
		len:   C.uintptr_t(len(str)),
		owned: false,
	}

	var once sync.Once // ensure we only free the string once

	free = func() {
		once.Do(func() {
			C.free(unsafe.Pointer(cstr))
		})
	}

	return symStr, free
}

/*
All cgo calls to symbolic functions which require error handling must be wrapped with the wrapCall function.

The symbolic Rust library defines a *thread-local* LAST_ERROR variable that contains the last error that occured in Rust code.
(see https://github.com/getsentry/symbolic/blob/d3877af86de642c18c799e9ec9c63381c91417f7/symbolic-cabi/src/utils.rs#L8-L10).

The error handling mechanism with symbolic is the following:
- clear LAST_ERROR with C.symbolic_err_clear()
- make a C.symbolic_xxx() call
- get the last error code with C.symbolic_err_get_last_code()

A call to runtime.LockOSThread ensures that the current goroutine will run on a single OS thread,
and that no other goroutine will be scheduled on the same OS thread (c.f. https://pkg.go.dev/runtime#LockOSThread),
which guarantees we have no race condition on the thread-local LAST_ERROR variable.
*/
func wrapCall(f func()) (err error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// clear last symbolic error
	C.symbolic_err_clear()

	// perform the call
	f()

	return getLastSymbolicError()
}

// Error represents an error from the execution of a C function
type Error struct {
	code    int64
	message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("symbolic error %d: %s", e.code, e.message)
}

func getLastSymbolicError() error {
	code := C.symbolic_err_get_last_code()
	if code == C.SYMBOLIC_ERROR_CODE_NO_ERROR {
		return nil
	}

	return &Error{
		code:    (int64)(code),
		message: decodeAndFreeString(C.symbolic_err_get_last_message()),
	}
}
