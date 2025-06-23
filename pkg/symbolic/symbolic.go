package symbolic

/*
	This file is a wrapper around the https://github.com/getsentry/symbolic library

	It uses the C-ABI bindings defined in https://github.com/getsentry/symbolic/tree/master/symbolic-cabi	To make it work locally, you need to follow the instructions on this page to compile the Rust library yourself
	and point the -L flag value to the directory containing the compiled binary
*/

// #cgo darwin LDFLAGS: -L ../../target/arm64-apple-darwin/release -l symbolic_cabi -lc++
// #cgo linux,amd64 LDFLAGS: -L ../../target/x86_64-unknown-linux/release -l symbolic_cabi -lstdc++ -lm
// #cgo linux,arm64 LDFLAGS: -L ../../target/aarch64-unknown-linux/release -l symbolic_cabi -lstdc++ -lm
// #include "../../symbolic-cabi/include/symbolic.h"
// #include "stdlib.h"
import "C"

import (
	"context"
	"errors"
	"math"
	"runtime"
	"sync"
	"unsafe"

	"github.com/google/uuid"
	// "go.ddbuild.io/dd-source/x/libs/go/log"
	// "go.ddbuild.io/dd-source/x/libs/go/statsd"
	// "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"github.com/DataDog/symbolic/pkg/null"
)

var (
	// ErrNoSuchObject occurs when trying to access a non-existing object in an archive
	ErrNoSuchObject = errors.New("no such object in the archive")

	// ErrLookupFailed occurs when a lookup returns no result
	ErrLookupFailed = errors.New("symbol lookup failed")
)

const demangleStringSizeInfoThreshold = 500
const demangleStringSizeHardLimit = 3000
const EXPECTED_SYMCACHE_VERSION = uint32(8)

type IL2CPPLineMapping struct {
	ptr   *C.SymbolicIL2CPPLineMapping
	bytes unsafe.Pointer
	mu    sync.Mutex
}

func IL2CPPLineMappingFromBytes(ctx context.Context, bytes []byte) (*IL2CPPLineMapping, error) {
	// We have to remember to manually free this C array.
	cbytes := C.CBytes(bytes)

	var ptr *C.SymbolicIL2CPPLineMapping
	err := wrapCall(func() {
		ptr = C.symbolic_il2cpp_line_mapping_from_bytes(
			(*C.uchar)(cbytes),
			C.uintptr_t(len(bytes)),
		)
	})
	if err != nil {
		C.free(cbytes)
		return nil, err
	}
	a := &IL2CPPLineMapping{ptr: ptr, bytes: cbytes}
	runtime.SetFinalizer(a, (*IL2CPPLineMapping).free)
	return a, nil
}

func (a *IL2CPPLineMapping) free() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := wrapCall(func() { C.symbolic_il2cpp_line_mapping_free(a.ptr) }); err != nil {
		// log.Error("error freeing line mapping", log.RichError(err))
	}
	C.free(a.bytes)
}

// Lookup performs a lookup for a given symbol
// This only returns the first matching symbol
func (a *IL2CPPLineMapping) Map(ctx context.Context, path string, line uint32) (_ string, _ uint32, err error) {
	// span, ctx := tracer.StartSpanFromContext(ctx, "IL2CPPLineMapping.Map", tracer.AnalyticsRate(1))
	// span.SetTag("path", path)
	// span.SetTag("line", line)
	// defer func() { span.Finish(tracer.WithError(err)) }()

	a.mu.Lock()
	defer a.mu.Unlock()

	var res C.SymbolicIL2CPPLineMappingResult
	err = wrapCall(func() {
		cPath := C.CString(path)
		defer C.free(unsafe.Pointer(cPath))
		res = C.symbolic_il2cpp_line_mapping_lookup(a.ptr, cPath, C.uint32_t(line))
	})
	runtime.KeepAlive(a)

	defer func() {
		if cleanErr := wrapCall(
			func() { C.symbolic_il2cpp_line_mapping_result_free(&res) }); cleanErr != nil {
			// log.Trace(ctx).Error("error freeing line mapping result", log.RichError(cleanErr))
		}
	}()
	if err != nil {
		return "", 0, err
	}

	resFile := C.GoStringN(res.file.data, C.int(res.file.len))
	if resFile == "" {
		err = errors.New("failed to map")
		return "", 0, err
	}
	return resFile, uint32(res.line), nil
}

// Archive represents a file, potentially containing multiple objects
type Archive struct {
	ptr   *C.SymbolicArchive
	bytes unsafe.Pointer
	mu    sync.Mutex
}

// ArchiveFromBytes loads an Archive from a []byte
func ArchiveFromBytes(ctx context.Context, bytes []byte) (*Archive, error) {
	// here we're allocating memory on the C heap, but it's our responsibility to release it
	// the bytes buffer might be used by symbolic during the lifetime of the object, so it should only be freed
	// when we release the archive
	cbytes := C.CBytes(bytes)

	var ptr *C.SymbolicArchive
	err := wrapCall(func() {
		ptr = C.symbolic_archive_from_bytes(
			(*C.uchar)(cbytes),
			C.uintptr_t(len(bytes)),
		)
	})
	if err != nil {
		C.free(cbytes)
		return nil, err
	}
	a := &Archive{ptr: ptr, bytes: cbytes}
	runtime.SetFinalizer(a, (*Archive).free)
	return a, nil
}

// free an Archive
func (a *Archive) free() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := wrapCall(func() { C.symbolic_archive_free(a.ptr) }); err != nil {
		// log.Warn("error freeing archive", log.RichError(err))
	}
	C.free(a.bytes)
}

// GetObjectCount returns the number of objects in the Archive
func (a *Archive) GetObjectCount(ctx context.Context) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	var count C.ulong
	err := wrapCall(func() {
		count = C.symbolic_archive_object_count(a.ptr)
	})
	runtime.KeepAlive(a)
	return int(count), err
}

// GetObject returns an object from an archive
func (a *Archive) GetObject(ctx context.Context, index int) (*Object, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	var ptr *C.SymbolicObject
	err := wrapCall(func() {
		ptr = C.symbolic_archive_get_object(
			a.ptr,
			C.uintptr_t(index),
		)
	})
	if err != nil {
		return nil, err
	}

	if ptr == nil {
		return nil, ErrNoSuchObject
	}
	o := &Object{ptr: ptr, archiveRef: a}
	runtime.SetFinalizer(o, (*Object).free)
	return o, nil
}

// GetObjectWithUUID returns the object in the symbol archive that has the given UUID
func (a *Archive) GetObjectWithUUID(ctx context.Context, id uuid.UUID) (*Object, error) {
	objCount, err := a.GetObjectCount(ctx)
	if err != nil {
		return nil, err
	}

	// go through the archives' objects and see if any matches the UUID
	for i := 0; i < objCount; i++ {
		obj, err := a.GetObject(ctx, i)
		if err != nil {
			return nil, err
		}
		objUUID, err := obj.GetCodeID(ctx)
		if err != nil {
			return nil, err
		}

		stdObjUUID, err := uuid.Parse(objUUID)
		if err != nil {
			return nil, err
		}
		if stdObjUUID != id {
			continue
		}
		return obj, nil
	}

	return nil, ErrNoSuchObject
}

// Object represents an object
type Object struct {
	ptr *C.SymbolicObject
	mu  sync.Mutex

	// reference to the archive from which this object comes
	// so that the archive doesn't get GC'd before this object, since the object uses the archive buffer
	archiveRef *Archive
}

// free an Object
func (o *Object) free() {
	o.mu.Lock()
	defer o.mu.Unlock()

	if err := wrapCall(func() { C.symbolic_object_free(o.ptr) }); err != nil {
		// log.Warn("error freeing object", log.RichError(err))
	}
}

// GetCodeID returns the code ID of the object
func (o *Object) GetCodeID(ctx context.Context) (string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	var uuid C.SymbolicStr
	err := wrapCall(func() {
		uuid = C.symbolic_object_get_code_id(o.ptr)
	})
	runtime.KeepAlive(o)

	return decodeAndFreeString(uuid), err
}

// ToSymCache creates a symcache from an object
func (o *Object) ToSymCache(ctx context.Context) (*SymCache, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	var ptr *C.SymbolicSymCache
	err := wrapCall(func() {
		ptr = C.symbolic_symcache_from_object(o.ptr)
	})
	runtime.KeepAlive(o)

	s := &SymCache{ptr: ptr}
	runtime.SetFinalizer(s, (*SymCache).free)
	return s, err
}

// SymCache represents a symbolic object cache
type SymCache struct {
	ptr   *C.SymbolicSymCache
	bytes unsafe.Pointer
	mu    sync.Mutex
}

// SymCacheFromBytes deserializes a symcache
func SymCacheFromBytes(ctx context.Context, bytes []byte) (*SymCache, error) {
	// here we're allocating memory on the C heap, but it's our responsibility to release it
	// the bytes buffer might be used by symbolic during the lifetime of the object, so it should only be freed
	// when we release the symcache
	cbytes := C.CBytes(bytes)

	var ptr *C.SymbolicSymCache
	err := wrapCall(func() {
		ptr = C.symbolic_symcache_from_bytes(
			(*C.uchar)(cbytes),
			C.uintptr_t(len(bytes)),
		)
	})
	if err != nil {
		C.free(cbytes)
		return nil, err
	}
	s := &SymCache{ptr: ptr, bytes: cbytes}
	runtime.SetFinalizer(s, (*SymCache).free)
	return s, nil
}

// free a symcache
func (s *SymCache) free() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := wrapCall(func() { C.symbolic_symcache_free(s.ptr) }); err != nil {
		// log.Warn("error freeing symcache", log.RichError(err))
	}
	C.free(s.bytes)
}

// GetDebugID returns the UUID of a symcache
func (s *SymCache) GetDebugID(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var uuid C.SymbolicStr
	err := wrapCall(func() {
		uuid = C.symbolic_symcache_get_debug_id(s.ptr)
	})
	runtime.KeepAlive(s)

	return decodeAndFreeString(uuid), err
}

// GetVersion returns the version of the symcache file
func (s *SymCache) GetVersion(ctx context.Context) (uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var version C.uint32_t
	err := wrapCall(func() {
		version = C.symbolic_symcache_get_version(s.ptr)
	})
	runtime.KeepAlive(s)
	return uint32(version), err
}

// GetLatestSymcacheVersion returns the symcache version generated by this version of Symbolic
func GetLatestSymcacheVersion(ctx context.Context) (uint32, error) {
	var version C.uint32_t
	err := wrapCall(func() {
		version = C.symbolic_symcache_latest_version()
	})
	return uint32(version), err
}

func (s *SymCache) Size(ctx context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var size C.ulong
	err := wrapCall(func() {
		size = C.symbolic_symcache_get_size(s.ptr)
	})
	if err != nil {
		return 0, err
	}

	return int64(size), nil
}

// ToBytes returns a serialized SymCache
func (s *SymCache) ToBytes(ctx context.Context) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var ptr *C.uchar
	err := wrapCall(func() {
		ptr = C.symbolic_symcache_get_bytes(s.ptr)
	})
	if err != nil {
		return []byte{}, err
	}

	var size C.ulong
	err = wrapCall(func() {
		size = C.symbolic_symcache_get_size(s.ptr)
	})
	runtime.KeepAlive(s)

	if err != nil {
		return []byte{}, err
	}

	return C.GoBytes(unsafe.Pointer(ptr), C.int(size)), nil
}

type SourceLocation struct {
	SymAddr   null.Uint64
	InstrAddr int
	Line      int
	Lang      string
	Symbol    null.String
	FullPath  string
}

// Lookup performs a lookup for a given symbol
// This only returns the first matching symbol
func (s *SymCache) Lookup(ctx context.Context, addr int) ([]SourceLocation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var res C.SymbolicLookupResult
	err := wrapCall(func() {
		res = C.symbolic_symcache_lookup(s.ptr, C.uint64_t(addr))
	})
	runtime.KeepAlive(s)

	defer func() {
		if err := wrapCall(func() { C.symbolic_lookup_result_free(&res) }); err != nil {
			// log.Trace(ctx).Warn("error freeing lookup result", log.RichError(err))
		}
	}()
	if err != nil {
		return []SourceLocation{}, err
	}
	if int(res.len) < 1 {
		return []SourceLocation{}, ErrLookupFailed
	}

	items := sourceLocationFromResult(res)
	return items, nil
}

func sourceLocationFromResult(res C.SymbolicLookupResult) []SourceLocation {
	elements := make([]SourceLocation, int(res.len))

	for i := 0; i < int(res.len); i++ {
		// This line assigns the i-th element of res.items to item
		// c.f. https://groups.google.com/g/golang-nuts/c/sV_f0VkjZTA
		// we compute a pointer at `res.items + (i * sizeof(one item))`
		item := (*C.SymbolicSourceLocation)(unsafe.Pointer(uintptr(unsafe.Pointer(res.items)) + unsafe.Sizeof(C.SymbolicSourceLocation{})*uintptr(i)))

		goSymAddr := uint64(item.sym_addr)
		var symAddrOption null.Uint64
		if goSymAddr == math.MaxUint32 {
			symAddrOption = null.Uint64{Valid: false}
		} else {
			symAddrOption = null.ValidUint64(goSymAddr)
		}

		goSymbol := C.GoStringN(item.symbol.data, C.int(item.symbol.len))
		var symbolOption null.String
		if goSymbol == "?" || goSymbol == "" {
			symbolOption = null.String{Valid: false}
		} else {
			symbolOption = null.ValidString(goSymbol)
		}

		elements[i] = SourceLocation{
			SymAddr:   symAddrOption,
			InstrAddr: int(item.instr_addr),
			Line:      int(item.line),
			Lang:      C.GoStringN(item.lang.data, C.int(item.lang.len)),
			Symbol:    symbolOption,
			FullPath:  C.GoStringN(item.full_path.data, C.int(item.full_path.len)),
		}
	}

	return elements
}

// Demangle tries to demangle an identifier
func Demangle(ctx context.Context, identifier string) (string, error) {
	// statsd.Distribution("dd.source_code_query.ios_demangle_length", float64(len(identifier)), nil, 1.0)
	if len(identifier) > demangleStringSizeHardLimit {
		// log.Trace(ctx).Warn("not demangling long identifier", log.String("symbol", identifier))
		return identifier, nil
	}

	// if len(identifier) > demangleStringSizeInfoThreshold {
	// 	log.Trace(ctx).Info("demangling identifier with length above info threshold", log.String("symbol", identifier))
	// }

	identifierStr, freeIdentifier := encodeString(identifier)
	defer freeIdentifier()

	langStr, freeLang := encodeString("") // empty language, we want it to be auto-detected
	defer freeLang()

	var demangled C.SymbolicStr
	err := wrapCall(func() {
		demangled = C.symbolic_demangle(&identifierStr, &langStr)
	})
	return decodeAndFreeString(demangled), err
}
